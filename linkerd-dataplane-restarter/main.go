package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	timeout = flag.DurationP("timeout", "t", 10*time.Minute, "how long wait for deleting pods")
	sleep   = flag.DurationP("sleep", "s", time.Minute, "how log wait between each deployment restart")
)

func main() {
	flag.Parse()

	ctx := context.Background()

	cli, err := getKubernetesCli()
	if err != nil {
		log.Fatal(err)
	}

	version, err := getCPlaneLinkerdVersion(ctx, cli)
	if err != nil {
		log.Fatal(err)
	}

	podsPerDep, err := getPodsPerDep(ctx, cli, version)
	if err != nil {
		log.Fatal(err)
	}

	if cont, err := shouldContinue(podsPerDep); err != nil {
		log.Fatal(err)
	} else if !cont {
		log.Printf("Nothing to do")
		return
	}

	for dep, podNames := range podsPerDep {
		if err := restartDep(ctx, dep); err != nil {
			log.Fatal(err)
		}

		if err := waitForDeletePods(ctx, podNames, *timeout); err != nil {
			log.Fatal(err)
		}

		time.Sleep(*sleep)
	}
}

func getKubernetesCli() (*kubernetes.Clientset, error) {
	kubeCfgPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build configuration from %q: %w", kubeCfgPath, err)
	}

	cli, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubernetes cli: %w", err)
	}
	return cli, err
}

func shouldContinue(podsPerDep map[string][]string) (bool, error) {
	if len(podsPerDep) == 0 {
		log.Printf("No deployments to be restarted")
		return false, nil
	}

	var depNames []string
	for dep := range podsPerDep {
		depNames = append(depNames, dep)
	}
	bb, err := json.MarshalIndent(depNames, "", "  ")
	if err != nil {
		return false, err
	}
	log.Printf("Deployments to be restarted (%d):\n%s", len(depNames), string(bb))

	return askForContinuation(), nil
}

func askForContinuation() bool {
	fmt.Printf("Continue? [Y]/n: ")
	input := bufio.NewScanner(os.Stdin)
	input.Scan()

	switch strings.ToLower(input.Text() + "y")[0] {
	case 'y', 't':
		return true
	default:
		return false
	}
}

func getPodsPerDep(ctx context.Context, cli *kubernetes.Clientset, version string) (map[string][]string, error) {
	podList, err := cli.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podsPerDep := map[string][]string{}
	for _, pod := range podList.Items {
		dep := pod.Labels["app"]
		if !shouldRestart(pod, version) || len(dep) == 0 {
			continue
		}

		name := "pod/" + pod.Name
		podsPerDep[dep] = append(podsPerDep[dep], name)
	}
	return podsPerDep, nil
}

func getCPlaneLinkerdVersion(ctx context.Context, cli *kubernetes.Clientset) (string, error) {
	dep, err := cli.AppsV1().Deployments("linkerd").Get(ctx, "linkerd-proxy-injector", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get linkerd-proxy-injector dep: %w", err)
	}

	ver := dep.Labels["app.kubernetes.io/version"]
	log.Printf("Desired linkerd version: %q", ver)
	return ver, nil
}

func shouldRestart(pod v1.Pod, desiredVersion string) bool {
	desiredVal := "linkerd/proxy-injector " + desiredVersion
	val := pod.Annotations["linkerd.io/created-by"]
	return val != "" && val != desiredVal
}

func restartDep(ctx context.Context, depName string) error {
	log.Printf("Start restarting %q", depName)

	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "restart", "deployment", depName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to restart %q: %w", depName, err)
	}

	log.Printf(string(output))
	return nil
}

func waitForDeletePods(ctx context.Context, pods []string, timeout time.Duration) error {
	log.Printf("Start waiting for %q", pods)

	args := append([]string{"wait", "--for=delete", "--timeout=" + timeout.String()}, pods...)
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to wait for pods %q: %w", pods, err)
	}

	log.Printf("Finish waiting for %q", pods)
	return nil
}

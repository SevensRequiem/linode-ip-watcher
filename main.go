package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/linode/linodego"
)

type Task struct {
	Action   func()
	Duration time.Duration
}

type Scheduler struct {
	StartTime          time.Time
	LastUpdate         time.Time
	LastUpdateDuration time.Duration
	Tasks              []Task
}

func main() {
	setIP()
	s15 := NewScheduler()
	s15.ScheduleTask(Task{
		Action: func() {
			setIP()
		},
		Duration: 15 * time.Minute,
	})
	go s15.Run()
	select {}
}

func setIP() error {
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	fmt.Println(currentTime, "Setting IP...")

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file:", err)
		return nil
	}

	linodeToken := os.Getenv("LINODE_TOKEN")
	if linodeToken == "" {
		fmt.Println("LINODE_TOKEN not set in .env file")
		return nil
	}

	resp, err := http.Get("http://ifconfig.me/ip")
	if err != nil {
		fmt.Println("Error getting IP address:", err)
		return nil
	}
	defer resp.Body.Close()

	ipAddr, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading IP address:", err)
		return nil
	}
	fmt.Println(currentTime, "IP Address:", string(ipAddr))

	client := linodego.NewClient(nil)
	client.SetToken(linodeToken)

	firewallStr := os.Getenv("FIREWALL")
	if firewallStr == "" {
		fmt.Errorf("FIREWALL not set in .env file")
		return nil
	}

	firewallID, err := strconv.Atoi(firewallStr)
	if err != nil {
		fmt.Println("Error converting firewall ID:", err)
		return nil
	}

	ctx := context.Background()
	firewallRules, err := client.GetFirewallRules(ctx, firewallID)
	if err != nil {
		fmt.Println("Error getting firewall rules:", err)
		return nil
	}

	label := os.Getenv("LABEL")
	if label == "" {
		fmt.Println("LABEL not set in .env file")
		return nil
	}
	for i, rule := range firewallRules.Inbound {
		if strings.HasPrefix(rule.Label, label) {
			fmt.Println(currentTime, "Updating firewall rule:", rule.Label)

			firewallRules.Inbound[i].Addresses = linodego.NetworkAddresses{
				IPv4: &[]string{string(ipAddr) + "/32"},
			}

			_, err := client.UpdateFirewallRules(ctx, firewallID, linodego.FirewallRuleSet{
				Inbound:        firewallRules.Inbound,
				Outbound:       firewallRules.Outbound,
				InboundPolicy:  "ACCEPT",
				OutboundPolicy: "ACCEPT",
			})
			if err != nil {
				fmt.Println("Error updating firewall rule:", err)
				continue
			}
			fmt.Println(currentTime, "Firewall rule updated:", rule.Label)
		}
	}

	return nil
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		StartTime: time.Now(),
		Tasks:     []Task{},
	}
}

func (s *Scheduler) ScheduleTask(task Task) {
	s.Tasks = append(s.Tasks, task)
}

func (s *Scheduler) Run() {
	s.StartTime = time.Now()
	fmt.Println("Scheduler started at:", s.StartTime.Format("2006-01-02 15:04:05"))

	for _, task := range s.Tasks {
		go func(t Task) {
			ticker := time.NewTicker(t.Duration)
			defer ticker.Stop()

			for range ticker.C {
				t.Action()
				s.LastUpdate = time.Now()
				s.LastUpdateDuration = s.LastUpdate.Sub(s.StartTime)
				fmt.Println("Task executed at:", s.LastUpdate.Format("2006-01-02 15:04:05"))
			}
		}(task)
	}
}

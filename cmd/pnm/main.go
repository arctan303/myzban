package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/pnm/proxy-node-manager/internal/api"
	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
	"github.com/pnm/proxy-node-manager/internal/installer"
	"github.com/pnm/proxy-node-manager/internal/proxy"
	"github.com/pnm/proxy-node-manager/internal/traffic"
	"github.com/pnm/proxy-node-manager/internal/user"
)

var (
	cfg         *config.Config
	database    *db.DB
	userService *user.Service
	xrayMgr     *proxy.XrayManager
	hy2Mgr      *proxy.Hy2Manager
	xrayInst    *installer.XrayInstaller
	hy2Inst     *installer.Hy2Installer
)

func initDeps() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err = db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	xrayMgr = proxy.NewXrayManager(cfg, database)
	hy2Mgr = proxy.NewHy2Manager(cfg, database)
	xrayInst = installer.NewXrayInstaller(cfg, database)
	hy2Inst = installer.NewHy2Installer(cfg, database)
	userService = user.NewService(database, cfg, xrayMgr, hy2Mgr)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "pnm",
		Short: "ProxyNode Manager — multi-user proxy management for VPS nodes",
	}

	// --- install ---
	installCmd := &cobra.Command{
		Use:   "install [vless|hy2|all]",
		Short: "Install proxy protocols",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			switch args[0] {
			case "vless":
				if err := xrayInst.Install(); err != nil {
					log.Fatalf("❌ %v", err)
				}
				// Generate initial config and start
				users, _ := database.ListEnabledUsers()
				xrayMgr.GenerateConfig(users)
				xrayMgr.Start()
				fmt.Println("✅ VLESS Reality installed and started!")

			case "hy2":
				if err := hy2Inst.Install(); err != nil {
					log.Fatalf("❌ %v", err)
				}
				// Generate Hy2 config and start
				users, _ := database.ListEnabledUsers()
				hy2Mgr.GenerateConfig(users)
				hy2Mgr.Start()
				fmt.Println("✅ Hysteria2 installed and started!")

			case "all":
				if err := xrayInst.Install(); err != nil {
					log.Printf("⚠ vless: %v", err)
				}
				if err := hy2Inst.Install(); err != nil {
					log.Printf("⚠ hy2: %v", err)
				}
				users, _ := database.ListEnabledUsers()
				xrayMgr.GenerateConfig(users)
				hy2Mgr.GenerateConfig(users)
				xrayMgr.Start()
				hy2Mgr.Start()
				fmt.Println("✅ All proxies installed and started!")

			default:
				log.Fatalf("Unknown protocol: %s (use vless, hy2, or all)", args[0])
			}
		},
	}

	// --- uninstall ---
	uninstallCmd := &cobra.Command{
		Use:   "uninstall [vless|hy2|all]",
		Short: "Uninstall proxy protocols",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			switch args[0] {
			case "vless":
				xrayInst.Uninstall()
				fmt.Println("✅ VLESS uninstalled")
			case "hy2":
				hy2Inst.Uninstall()
				fmt.Println("✅ Hysteria2 uninstalled")
			case "all":
				xrayInst.Uninstall()
				hy2Inst.Uninstall()
				fmt.Println("✅ All proxies uninstalled")
			}
		},
	}

	// --- proxy status ---
	proxyCmd := &cobra.Command{
		Use:   "proxy",
		Short: "Proxy service management",
	}

	proxyStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show proxy status",
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			nodeInfo, _ := database.GetNodeInfo()
			vlessConf, _ := database.GetProxyConfig("vless")
			hy2Conf, _ := database.GetProxyConfig("hysteria2")

			fmt.Println("╔════════════════════════════════════════╗")
			fmt.Println("║       ProxyNode Manager Status        ║")
			fmt.Println("╚════════════════════════════════════════╝")
			if nodeInfo != nil && nodeInfo.ServerIP != "" {
				fmt.Printf("  Server IP:  %s\n", nodeInfo.ServerIP)
			}
			fmt.Println()

			// VLESS
			installed := vlessConf != nil && vlessConf.Installed
			running := xrayMgr.IsRunning()
			fmt.Printf("  VLESS Reality:\n")
			fmt.Printf("    Installed: %s\n", boolIcon(installed))
			fmt.Printf("    Running:   %s\n", boolIcon(running))
			if installed {
				fmt.Printf("    Port:      %d (TCP)\n", cfg.VLESSPort)
			}
			fmt.Println()

			// Hy2
			installed = hy2Conf != nil && hy2Conf.Installed
			running = hy2Mgr.IsRunning()
			fmt.Printf("  Hysteria2:\n")
			fmt.Printf("    Installed: %s\n", boolIcon(installed))
			fmt.Printf("    Running:   %s\n", boolIcon(running))
			if installed {
				fmt.Printf("    Port:      %d (UDP)\n", cfg.Hy2Port)
			}
		},
	}

	proxyStartCmd := &cobra.Command{
		Use:   "start [vless|hy2|all]",
		Short: "Start a proxy service",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			users, _ := database.ListEnabledUsers()

			switch args[0] {
			case "vless":
				xrayMgr.GenerateConfig(users)
				if err := xrayMgr.Start(); err != nil {
					log.Fatalf("❌ %v", err)
				}
				fmt.Println("✅ VLESS started")
			case "hy2":
				hy2Mgr.GenerateConfig(users)
				if err := hy2Mgr.Start(); err != nil {
					log.Fatalf("❌ %v", err)
				}
				fmt.Println("✅ Hysteria2 started")
			case "all":
				xrayMgr.GenerateConfig(users)
				hy2Mgr.GenerateConfig(users)
				xrayMgr.Start()
				hy2Mgr.Start()
				fmt.Println("✅ All proxies started")
			}
		},
	}

	proxyStopCmd := &cobra.Command{
		Use:   "stop [vless|hy2|all]",
		Short: "Stop a proxy service",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			switch args[0] {
			case "vless":
				xrayMgr.Stop()
				fmt.Println("✅ VLESS stopped")
			case "hy2":
				hy2Mgr.Stop()
				fmt.Println("✅ Hysteria2 stopped")
			case "all":
				xrayMgr.Stop()
				hy2Mgr.Stop()
				fmt.Println("✅ All proxies stopped")
			}
		},
	}

	proxyCmd.AddCommand(proxyStatusCmd, proxyStartCmd, proxyStopCmd)

	// --- user ---
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management",
	}

	userAddCmd := &cobra.Command{
		Use:   "add <username>",
		Short: "Add a new user (auto-generates credentials)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			u, err := userService.CreateUser(args[0])
			if err != nil {
				log.Fatalf("❌ %v", err)
			}

			fmt.Println("✅ User created successfully!")
			fmt.Println()
			fmt.Printf("  Username:      %s\n", u.Username)
			fmt.Printf("  Email:         %s\n", u.Email)
			fmt.Printf("  VLESS UUID:    %s\n", u.UUID)
			fmt.Printf("  Hy2 Password:  %s\n", u.Hy2Password)
			fmt.Printf("  Enabled:       %s\n", boolIcon(u.Enabled))
			fmt.Println()
			fmt.Println("Use 'pnm user info <username>' to see client config.")
		},
	}

	userListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			users, err := userService.ListUsers()
			if err != nil {
				log.Fatalf("❌ %v", err)
			}

			if len(users) == 0 {
				fmt.Println("No users. Use 'pnm user add <username>' to create one.")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tUSERNAME\tENABLED\tUPLOAD\tDOWNLOAD\tTOTAL")
			fmt.Fprintln(w, "──\t────────\t───────\t──────\t────────\t─────")
			for _, u := range users {
				total := u.TrafficUp + u.TrafficDown
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
					u.ID, u.Username, boolIcon(u.Enabled),
					api.FormatBytes(u.TrafficUp),
					api.FormatBytes(u.TrafficDown),
					api.FormatBytes(total),
				)
			}
			w.Flush()
		},
	}

	userInfoCmd := &cobra.Command{
		Use:   "info <username>",
		Short: "Show user details and client config",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			u, err := userService.GetUser(args[0])
			if err != nil {
				log.Fatalf("❌ %v", err)
			}

			total := u.TrafficUp + u.TrafficDown
			fmt.Println("╔════════════════════════════════════════╗")
			fmt.Printf("║  User: %-32s║\n", u.Username)
			fmt.Println("╚════════════════════════════════════════╝")
			fmt.Printf("  ID:            %d\n", u.ID)
			fmt.Printf("  Email:         %s\n", u.Email)
			fmt.Printf("  VLESS UUID:    %s\n", u.UUID)
			fmt.Printf("  Hy2 Password:  %s\n", u.Hy2Password)
			fmt.Printf("  Enabled:       %s\n", boolIcon(u.Enabled))
			fmt.Printf("  Upload:        %s\n", api.FormatBytes(u.TrafficUp))
			fmt.Printf("  Download:      %s\n", api.FormatBytes(u.TrafficDown))
			fmt.Printf("  Total:         %s\n", api.FormatBytes(total))
			if u.TrafficLimit > 0 {
				fmt.Printf("  Limit:         %s\n", api.FormatBytes(u.TrafficLimit))
			}
			if u.ExpiresAt != nil {
				fmt.Printf("  Expires:       %s\n", u.ExpiresAt.Format("2006-01-02 15:04:05"))
			}
			fmt.Println()

			// Output client config
			clientCfg, err := userService.GetClientConfig(args[0])
			if err != nil {
				fmt.Printf("  ⚠ No client config: %v\n", err)
				return
			}

			fmt.Println("── Clash / Meta 客户端配置 (YAML) ──────────")
			fmt.Println("proxies:")
			fmt.Println(clientCfg)
			fmt.Println("─────────────────────────────────────────────")
		},
	}

	userEnableCmd := &cobra.Command{
		Use:   "enable <username>",
		Short: "Enable a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()
			if err := userService.EnableUser(args[0]); err != nil {
				log.Fatalf("❌ %v", err)
			}
			fmt.Printf("✅ User '%s' enabled\n", args[0])
		},
	}

	userDisableCmd := &cobra.Command{
		Use:   "disable <username>",
		Short: "Disable a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()
			if err := userService.DisableUser(args[0]); err != nil {
				log.Fatalf("❌ %v", err)
			}
			fmt.Printf("✅ User '%s' disabled\n", args[0])
		},
	}

	userDeleteCmd := &cobra.Command{
		Use:   "delete <username>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()
			if err := userService.DeleteUser(args[0]); err != nil {
				log.Fatalf("❌ %v", err)
			}
			fmt.Printf("✅ User '%s' deleted\n", args[0])
		},
	}

	userCmd.AddCommand(userAddCmd, userListCmd, userInfoCmd, userEnableCmd, userDisableCmd, userDeleteCmd)

	// --- traffic ---
	trafficCmd := &cobra.Command{
		Use:   "traffic",
		Short: "Traffic statistics",
	}

	trafficShowCmd := &cobra.Command{
		Use:   "show [username]",
		Short: "Show traffic statistics",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			if len(args) == 1 {
				// Show specific user
				u, err := userService.GetUser(args[0])
				if err != nil {
					log.Fatalf("❌ %v", err)
				}
				fmt.Printf("User: %s\n", u.Username)
				fmt.Printf("  Upload:   %s\n", api.FormatBytes(u.TrafficUp))
				fmt.Printf("  Download: %s\n", api.FormatBytes(u.TrafficDown))
				fmt.Printf("  Total:    %s\n", api.FormatBytes(u.TrafficUp+u.TrafficDown))

				logs, _ := userService.GetTrafficLogs(args[0], 20)
				if len(logs) > 0 {
					fmt.Println("\n  Recent traffic logs:")
					for _, l := range logs {
						fmt.Printf("    %s  %s  ↑%s ↓%s\n",
							l.RecordAt.Format("01-02 15:04"),
							strings.ToUpper(l.Protocol),
							api.FormatBytes(l.Upload),
							api.FormatBytes(l.Download),
						)
					}
				}
			} else {
				// Show all users summary
				users, err := userService.ListUsers()
				if err != nil {
					log.Fatalf("❌ %v", err)
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "USERNAME\tUPLOAD\tDOWNLOAD\tTOTAL\tLIMIT")
				fmt.Fprintln(w, "────────\t──────\t────────\t─────\t─────")
				for _, u := range users {
					limit := "∞"
					if u.TrafficLimit > 0 {
						limit = api.FormatBytes(u.TrafficLimit)
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						u.Username,
						api.FormatBytes(u.TrafficUp),
						api.FormatBytes(u.TrafficDown),
						api.FormatBytes(u.TrafficUp+u.TrafficDown),
						limit,
					)
				}
				w.Flush()
			}
		},
	}

	trafficResetCmd := &cobra.Command{
		Use:   "reset <username>",
		Short: "Reset traffic counters for a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()
			if err := userService.ResetTraffic(args[0]); err != nil {
				log.Fatalf("❌ %v", err)
			}
			fmt.Printf("✅ Traffic reset for '%s'\n", args[0])
		},
	}

	trafficCmd.AddCommand(trafficShowCmd, trafficResetCmd)

	// --- serve ---
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the API server + traffic collector (daemon mode)",
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()

			fmt.Println("╔════════════════════════════════════════╗")
			fmt.Println("║    ProxyNode Manager — Daemon Mode    ║")
			fmt.Println("╚════════════════════════════════════════╝")

			// Start traffic collector
			collector := traffic.NewCollector(database, xrayMgr, hy2Mgr, cfg.CollectInterval)
			collector.Start()

			// Start Hy2 auth endpoint in background
			go func() {
				server := api.NewServer(cfg, database, userService, xrayMgr, hy2Mgr, xrayInst, hy2Inst)
				if err := server.StartAuthEndpoint(); err != nil {
					log.Printf("[auth] endpoint error: %v", err)
				}
			}()

			// Start API server in background
			go func() {
				server := api.NewServer(cfg, database, userService, xrayMgr, hy2Mgr, xrayInst, hy2Inst)
				if err := server.Start(); err != nil {
					log.Fatalf("[api] server error: %v", err)
				}
			}()

			fmt.Printf("  API:        http://%s\n", cfg.APIListenAddr)
			fmt.Printf("  Hy2 Auth:   http://%s/hy2/auth\n", cfg.AuthListenAddr)
			fmt.Printf("  Collector:  every %ds\n", cfg.CollectInterval)
			fmt.Println()
			fmt.Println("Press Ctrl+C to stop.")

			// Wait for shutdown signal
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			fmt.Println("\nShutting down...")
			collector.Stop()
			database.Close()
		},
	}

	rootCmd.AddCommand(installCmd, uninstallCmd, proxyCmd, userCmd, trafficCmd, serveCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func boolIcon(b bool) string {
	if b {
		return "✅"
	}
	return "❌"
}

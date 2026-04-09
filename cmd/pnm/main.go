package main

import (
	"crypto/rand"
	"encoding/hex"
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
				users, _ := database.ListEnabledUsers()
				xrayMgr.GenerateConfig(users)
				xrayMgr.Start()
				fmt.Println("✅ VLESS Reality installed and started!")
			case "hy2":
				if err := hy2Inst.Install(); err != nil {
					log.Fatalf("❌ %v", err)
				}
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

	// --- token ---
	tokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Manage admin API token",
	}

	tokenInitCmd := &cobra.Command{
		Use:   "init",
		Short: "Generate and set a new admin API token",
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			token := generateAdminToken()
			nodeInfo, _ := database.GetNodeInfo()
			if nodeInfo == nil {
				log.Fatal("node info not found")
			}
			nodeInfo.AdminToken = token
			if err := database.SaveNodeInfo(nodeInfo); err != nil {
				log.Fatalf("❌ save token: %v", err)
			}

			fmt.Println("✅ Admin token generated!")
			fmt.Println()
			fmt.Printf("  Token: %s\n", token)
			fmt.Println()
			ip := nodeInfo.ServerIP
			if ip == "" {
				ip = "YOUR_SERVER_IP"
			}
			fmt.Printf("  API URL: http://%s:9090/api/v1/status?token=%s\n", ip, token)
			fmt.Println()
			fmt.Println("⚠  Save this token! You'll need it to connect the panel.")
			fmt.Println("   The API is now protected — all /api/v1/* requests require this token.")
		},
	}

	tokenShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current admin API token",
		Run: func(cmd *cobra.Command, args []string) {
			initDeps()
			defer database.Close()

			nodeInfo, _ := database.GetNodeInfo()
			if nodeInfo == nil || nodeInfo.AdminToken == "" {
				fmt.Println("No admin token set. Run: pnm token init")
				return
			}
			fmt.Printf("Admin Token: %s\n", nodeInfo.AdminToken)
		},
	}

	tokenCmd.AddCommand(tokenInitCmd, tokenShowCmd)

	// --- proxy ---
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
			if nodeInfo != nil && nodeInfo.AdminToken != "" {
				fmt.Printf("  Admin API:  http://%s:9090/api/v1/\n", nodeInfo.ServerIP)
			}
			fmt.Println()

			installed := vlessConf != nil && vlessConf.Installed
			running := xrayMgr.IsRunning()
			fmt.Printf("  VLESS Reality:\n")
			fmt.Printf("    Installed: %s\n", boolIcon(installed))
			fmt.Printf("    Running:   %s\n", boolIcon(running))
			if installed {
				fmt.Printf("    Port:      %d (TCP)\n", cfg.VLESSPort)
			}
			fmt.Println()

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

			subURL, _ := userService.GetSubscriptionURL(args[0])

			fmt.Println("✅ User created successfully!")
			fmt.Println()
			fmt.Printf("  Username:      %s\n", u.Username)
			fmt.Printf("  VLESS UUID:    %s\n", u.UUID)
			fmt.Printf("  Hy2 Password:  %s\n", u.Hy2Password)
			fmt.Printf("  Sub Token:     %s\n", u.SubToken)
			fmt.Printf("  Enabled:       %s\n", boolIcon(u.Enabled))
			fmt.Println()
			if subURL != "" {
				fmt.Printf("  📋 Subscription URL: %s\n", subURL)
				fmt.Println("     (User can import this URL in Clash/v2ray client)")
			}
			fmt.Println()
			fmt.Println("Use 'pnm user info <username>' to see full client config.")
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

			subURL, _ := userService.GetSubscriptionURL(args[0])
			total := u.TrafficUp + u.TrafficDown

			fmt.Println("╔════════════════════════════════════════╗")
			fmt.Printf("║  User: %-32s║\n", u.Username)
			fmt.Println("╚════════════════════════════════════════╝")
			fmt.Printf("  ID:            %d\n", u.ID)
			fmt.Printf("  Email:         %s\n", u.Email)
			fmt.Printf("  VLESS UUID:    %s\n", u.UUID)
			fmt.Printf("  Hy2 Password:  %s\n", u.Hy2Password)
			fmt.Printf("  Sub Token:     %s\n", u.SubToken)
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
			if subURL != "" {
				fmt.Printf("\n  📋 Subscription URL: %s\n", subURL)
			}
			fmt.Println()

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

			nodeInfo, _ := database.GetNodeInfo()
			fmt.Println("╔════════════════════════════════════════╗")
			fmt.Println("║    ProxyNode Manager — Daemon Mode    ║")
			fmt.Println("╚════════════════════════════════════════╝")
			if nodeInfo != nil && nodeInfo.AdminToken == "" {
				fmt.Println("⚠  No admin token set! Run 'pnm token init' first.")
				fmt.Println("   API will reject all requests until a token is set.")
			}

			collector := traffic.NewCollector(database, xrayMgr, hy2Mgr, cfg.CollectInterval)
			collector.Start()

			go func() {
				server := api.NewServer(cfg, database, userService, xrayMgr, hy2Mgr, xrayInst, hy2Inst)
				if err := server.StartAuthEndpoint(); err != nil {
					log.Printf("[auth] endpoint error: %v", err)
				}
			}()

			go func() {
				server := api.NewServer(cfg, database, userService, xrayMgr, hy2Mgr, xrayInst, hy2Inst)
				if err := server.Start(); err != nil {
					log.Fatalf("[api] server error: %v", err)
				}
			}()

			fmt.Printf("  API:        http://%s (admin token required)\n", cfg.APIListenAddr)
			fmt.Printf("  Sub URL:    http://%s:9090/sub/{token}\n", nodeInfo.ServerIP)
			fmt.Printf("  Hy2 Auth:   http://%s/hy2/auth\n", cfg.AuthListenAddr)
			fmt.Printf("  Collector:  every %ds\n", cfg.CollectInterval)
			fmt.Println()
			fmt.Println("Press Ctrl+C to stop.")

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			fmt.Println("\nShutting down...")
			collector.Stop()
			database.Close()
		},
	}

	rootCmd.AddCommand(installCmd, uninstallCmd, tokenCmd, proxyCmd, userCmd, trafficCmd, serveCmd)

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

func generateAdminToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

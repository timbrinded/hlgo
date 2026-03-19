package cmd

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newAccountApproveAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve-agent",
		Short: "Approve or revoke an agent wallet",
		Long: `Approve an agent address to trade on behalf of your account.

Note: Hyperliquid may charge an activation fee when first approving an agent.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			agent, _ := cmd.Flags().GetString("agent") //nolint:errcheck // known flag
			name, _ := cmd.Flags().GetString("name")   //nolint:errcheck // known flag
			revoke, _ := cmd.Flags().GetBool("revoke") //nolint:errcheck // known flag

			if !common.IsHexAddress(agent) {
				return output.NewCLIError(output.ErrValidation, "invalid agent address").
					WithDetails("agent", agent)
			}

			agentName := strings.TrimSpace(name)
			switch {
			case revoke:
				if err := requireConfirm("approve-agent --revoke", confirmationAccepted(cmd), cfg.DryRun); err != nil {
					return err
				}
				agentName = ""
			case agentName == "":
				return output.NewCLIError(output.ErrValidation, "agent name is required unless --revoke is set").
					WithDetails("hint", "set --name <1-16 chars> to approve, or use --revoke with --confirm to revoke")
			case len(agentName) > 16:
				return output.NewCLIError(output.ErrValidation, "agent name must be at most 16 characters").
					WithDetails("value", agentName).
					WithDetails("max_length", 16)
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			input := exchange.ApproveAgentInput{
				AgentAddress: strings.ToLower(agent),
				AgentName:    agentName,
				DryRun:       cfg.DryRun,
			}
			raw, err := exec.ApproveAgent(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("agent", "", "agent wallet address")
	cmd.Flags().String("name", "", "optional agent label")
	cmd.Flags().Bool("revoke", false, "revoke agent by setting an empty label")
	cmd.Flags().Bool("confirm", false, "confirm execution for revoke flow")
	cmd.Flags().Bool("yes", false, "alias for --confirm")
	mustMarkRequiredFlags(cmd, "agent")

	return cmd
}

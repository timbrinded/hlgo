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
		Long: `Approve an agent address to trade on behalf of the master wallet.

Note: Hyperliquid may charge an activation fee when first approving an agent.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			agent, _ := cmd.Flags().GetString("agent")   //nolint:errcheck // known flag
			name, _ := cmd.Flags().GetString("name")     //nolint:errcheck // known flag
			revoke, _ := cmd.Flags().GetBool("revoke")   //nolint:errcheck // known flag
			confirm, _ := cmd.Flags().GetBool("confirm") //nolint:errcheck // known flag
			yes, _ := cmd.Flags().GetBool("yes")         //nolint:errcheck // known flag

			if !common.IsHexAddress(agent) {
				return output.NewCLIError(output.ErrValidation, "invalid agent address").
					WithDetails("agent", agent)
			}

			agentName := name
			if revoke {
				if err := requireConfirm("approve-agent --revoke", confirm || yes, cfg.DryRun); err != nil {
					return err
				}
				agentName = ""
			}

			exec, err := buildMasterExecutor(cfg)
			if err != nil {
				return err
			}

			raw, err := exec.ApproveAgent(cmd.Context(), exchange.ApproveAgentInput{
				AgentAddress: strings.ToLower(agent),
				AgentName:    agentName,
				DryRun:       cfg.DryRun,
			})
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
	//nolint:errcheck // MarkFlagRequired on known flags never fails
	cmd.MarkFlagRequired("agent")

	return cmd
}

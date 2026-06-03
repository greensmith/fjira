package commands

import (
	"github.com/mk-5/fjira/internal/fjira"
	"github.com/mk-5/fjira/internal/workspaces"
	"github.com/spf13/cobra"
)

func GetJqlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "jql [query]",
		Short: "Search using custom JQL queries",
		Long:  "Opens JQL search. Pass a JQL string as argument to search immediately, or omit to enter JQL interactively.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			s := cmd.Context().Value(CtxWorkspaceSettings).(*workspaces.WorkspaceSettings)
			f := fjira.CreateNewFjira(s)
			defer f.Close()
			cliArgs := &fjira.CliArgs{
				JqlMode: true,
			}
			if len(args) == 1 {
				cliArgs.JqlQuery = args[0]
			}
			f.Run(cliArgs)
		},
	}
}

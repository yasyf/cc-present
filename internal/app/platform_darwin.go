package app

import "github.com/yasyf/daemonkit"

const (
	daemonTeamID            = "SXKCTF23Q2"
	daemonSigningIdentifier = "cc-present"
)

// daemonTrust pins the control lane to the signed cc-present build, so drain
// and broker-handoff admit nothing else; the business lane keeps the same-EUID
// floor a CLI's unsigned dev build has always run under.
func daemonTrust() daemonkit.Trust {
	requirement := daemonkit.Requirement{TeamID: daemonTeamID, SigningIdentifier: daemonSigningIdentifier}
	return daemonkit.Trust{Control: &requirement, Serving: daemonkit.ServingSameUser()}
}

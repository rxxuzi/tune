package server

import (
	"encoding/json"
)

func parseJSONToSSHInfo(data []byte) (SSHInfo, error) {
	var info SSHInfo
	err := json.Unmarshal(data, &info)
	return info, err
}

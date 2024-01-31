package app

import (
	"encoding/json"
	"zvrtsh/internal/ports/httpserver"
)

// костыльные попытки скастовать дату к нормальному типу

func getSecretCodeFromData(m any) string {
	return m.(map[string]any)["secret_code"].(string)
}

func getInviteCodeFromData(m any) string {
	return m.(map[string]any)["invite_code"].(string)
}

func getSessionIdFromData(m any) string {
	return m.(map[string]any)["session_id"].(string)
}

func getStepsAmountFromData(m any) int {
	return int(m.(map[string]any)["steps_amount"].(float64))
}

func getOpenQuestionFromData(m any) string {
	return m.(map[string]any)["open_question"].(string)
}

func getResultsFromData(m any) httpserver.GetSessionData {
	data, _ := json.Marshal(m)
	var results httpserver.GetSessionData
	_ = json.Unmarshal(data, &results)
	return results
}

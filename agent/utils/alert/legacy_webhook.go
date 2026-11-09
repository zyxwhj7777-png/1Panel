package alert

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/dto"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/constant"
	"github.com/1Panel-dev/1Panel/agent/utils/webhook_sender"
)

// CreateLegacyWebhookAlertLog delivers an alert through the built-in WeCom/DingTalk/FeiShu
// webhooks. Unlike the templated custom webhook, those channels keep a plain
// {displayName, url} config and use the fixed payload builders in utils/webhook_sender.
func CreateLegacyWebhookAlertLog(
	alertType string,
	info dto.AlertDTO,
	create dto.AlertLogCreate,
	project string,
	params []dto.Param,
	config model.AlertConfig,
	transport *http.Transport,
	agentInfo *dto.AgentInfo,
) error {
	alertInfo := info
	alertInfo.Type = alertType
	create.Type = GetCronJobType(alertType)
	create.AlertRule = ProcessAlertRule(info)
	create.AlertDetail = ProcessAlertDetail(alertInfo, project, params, config.Type)
	return deliverLegacyWebhook(create, config, transport, agentInfo, customWebhookNow())
}

// CreateTaskScanLegacyWebhookAlertLog is the task-scan variant of CreateLegacyWebhookAlertLog.
func CreateTaskScanLegacyWebhookAlertLog(
	info dto.AlertDTO,
	alertType string,
	create dto.AlertLogCreate,
	pushAlert dto.PushAlert,
	config model.AlertConfig,
	transport *http.Transport,
	agentInfo *dto.AgentInfo,
) error {
	params := CreateTaskAlertParams(pushAlert)
	alertInfo := info
	alertInfo.Type = alertType
	create.Type = GetCronJobType(alertType)
	create.AlertRule = ProcessAlertRule(info)
	create.AlertDetail = ProcessAlertDetail(alertInfo, pushAlert.TaskName, params, config.Type)
	return deliverLegacyWebhook(create, config, transport, agentInfo, customWebhookNow())
}

func deliverLegacyWebhook(
	create dto.AlertLogCreate,
	config model.AlertConfig,
	transport *http.Transport,
	agentInfo *dto.AgentInfo,
	occurredAt time.Time,
) error {
	var alertLog model.AlertLog
	data, err := customWebhookTemplateData(create.AlertDetail, agentInfo, occurredAt)
	if err != nil {
		return saveCustomWebhookDeliveryError(create, &alertLog, err)
	}
	url, err := legacyWebhookURL(config)
	if err != nil {
		return saveCustomWebhookDeliveryError(create, &alertLog, err)
	}
	payload, err := webhook_sender.BuildWebhookPayload(config.Type, data.Title, data.Message)
	if err != nil {
		return saveCustomWebhookDeliveryError(create, &alertLog, err)
	}
	if err := webhook_sender.SendWebhookRequest(config.Type, url, payload, transport); err != nil {
		return saveCustomWebhookDeliveryError(create, &alertLog, err)
	}
	create.Status = constant.AlertSuccess
	return SaveAlertLog(create, &alertLog)
}

func legacyWebhookURL(config model.AlertConfig) (string, error) {
	var item struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(config.Config), &item); err != nil {
		return "", errors.New("resolve webhook url failed")
	}
	url := strings.TrimSpace(item.URL)
	if url == "" {
		return "", errors.New("webhook url is empty")
	}
	return url, nil
}
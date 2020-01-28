package utilities

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"pipeline-operator/pkg/apis/algorun/v1beta1"
	"strings"
)

var notifDisabled = false

func Notify(notifMessage *v1beta1.NotifMessage) {

	if !notifDisabled {

		notifEnvURL := os.Getenv("NOTIF_URL")
		if notifEnvURL == "" {
			notifDisabled = true
			log.Error(errors.New("NOTIF_URL environment variable must be set in order to send notifications"), "Notifications disabled")
			return
		}

		logData := map[string]interface{}{
			"NotifType": &notifMessage.Type_,
		}

		notifLogger := log.WithValues("data", logData)

		u, _ := url.Parse(fmt.Sprintf("%s/%schanged", notifEnvURL, strings.ToLower(notifMessage.Type_)))
		notifURL := u.String()

		jsonValue, _ := json.Marshal(notifMessage)
		req, err := http.NewRequest("POST", notifURL, bytes.NewReader(jsonValue))
		req.Header.Set("Content-Type", "application/json")

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err := client.Do(req)
		if err != nil {
			notifLogger.Error(err, "Error sending notification.")
			return
		}
		defer resp.Body.Close()

	}

}

package utilities

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"endpoint-operator/pkg/apis/algo/v1alpha1"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var notifDisabled = false

func Notify(notifMessage *v1alpha1.NotifMessage) {

	if !notifDisabled {

		notifEnvURL := os.Getenv("NOTIF_URL")
		if notifEnvURL == "" {
			notifDisabled = true
			log.Error(errors.New("NOTIF_URL environment variable must be set in order to send notifications"), "Notifications disabled")
			return
		}

		notifLogger := log.WithValues("NotifType", &notifMessage.LogMessageType)

		u, _ := url.Parse(fmt.Sprintf("%s/%schanged", notifEnvURL, strings.ToLower(notifMessage.LogMessageType)))
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

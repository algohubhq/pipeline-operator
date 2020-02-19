package utilities

import (
	"os"
	"strconv"
)

// CheckForKafkaTLS checks for the KAFKA-TLS envar and certs
func GetKafkaClusterName() string {

	// Try to load from environment variable
	return os.Getenv("KAFKA-CLUSTER-NAME")

}

// CheckForKafkaTLS checks for the KAFKA-TLS envar and certs
func CheckForKafkaTLS() bool {

	// Try to load from environment variable
	kafkaTLSEnv := os.Getenv("KAFKA-TLS")
	if kafkaTLSEnv == "" {
		return false
	}

	kafkaTLS, err := strconv.ParseBool(kafkaTLSEnv)
	if err != nil {
		return false
	}

	return kafkaTLS

}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

/*
   Status provides status information on experiments
   Copyright (C) 2023 Timothy Drysdale <timothy.d.drysdale@gmail.com>

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as
   published by the Free Software Foundation, either version 3 of the
   License, or (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package cmd

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" //ok in production https://medium.com/google-cloud/continuous-profiling-of-go-programs-96d4416af77b
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/practable/status/internal/config"
	"github.com/practable/status/internal/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

/* configuration

bufferSize
muxBufferLength (for main message queue into the mux)
clientBufferLength (for each client's outgoing channel)

*/

// rootCmd represents the base command when called without any subcommands
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "status server with REST-like API",
	Long: `status server provides a summar of experiment statuses over a REST-like API. Set parameters with environment
variables, for example:

export STATUS_BASEPATH_BOOK=/tenant/book
export STATUS_BASEPATH_JUMP=/tenant/jump
export STATUS_BASEPATH_RELAY=/tenant/relay
export STATUS_EMAIL_AUTH_TYPE=plain
export STATUS_EMAIL_CC="beta@a.org"
export STATUS_EMAIL_FROM=other@b.org
export STATUS_EMAIL_HOST=stmp.b.org
export STATUS_EMAIL_LINK=app.practable.io/tenant/status
export STATUS_EMAIL_PASSWORD=something
export STATUS_EMAIL_PORT=587
export STATUS_EMAIL_SUBJECT=app.practable.io/tenant
export STATUS_EMAIL_TO="alpha@a.org"
export STATUS_HEALTH_LAST_ACTIVE=10s
export STATUS_HEALTH_LAST_CHECKED=30s
export STATUS_HEALTH_LOG_EVERY=10m
export STATUS_HEALTH_EVENTS=100
export STATUS_HEALTH_STARTUP=1m
export STATUS_HOST_BOOK=app.practable.io
export STATUS_HOST_JUMP=app.practable.io
export STATUS_HOST_RELAY=app.practable.io
export STATUS_LOG_LEVEL=warn
export STATUS_LOG_FORMAT=json
export STATUS_LOG_FILE=/var/log/status/status.log
export STATUS_PORT_PROFILE=6061
export STATUS_PORT_SERVE=3007
export STATUS_PROFILE=false
export STATUS_QUERY_BOOK_EVERY=15m
export STATUS_RECONNECT_JUMP_EVERY=1d
export STATUS_RECONNECT_RELAY_EVERY=1d
export STATUS_SCHEME_BOOK=https
export STATUS_SCHEME_JUMP=https
export STATUS_SCHEME_RELAY=https
export STATUS_SECRET_JUMP=othersecret
export STATUS_SECRET_BOOK=somesecret
export STATUS_SECRET_RELAY=somesecret
export STATUS_TIMEOUT_BOOK=5s
status serve 

`,
	Run: func(cmd *cobra.Command, args []string) {

		viper.SetEnvPrefix("STATUS")
		viper.AutomaticEnv()

		viper.SetDefault("basepath_book", "/") // "" so we can check it's been provided
		viper.SetDefault("basepath_jump", "/")
		viper.SetDefault("basepath_relay", "/")

		viper.SetDefault("email_auth_type", "none")
		viper.SetDefault("email_cc", []string{})
		viper.SetDefault("email_from", "") // "" so we can check it's been provided
		viper.SetDefault("email_host", "")
		viper.SetDefault("email_link", "")
		viper.SetDefault("email_password", "")
		viper.SetDefault("email_port", 587)
		viper.SetDefault("email_subject", "")
		viper.SetDefault("email_to", []string{})

		viper.SetDefault("health_last_active", "10s")  // last stream Tx should be more recent than this
		viper.SetDefault("health_last_checked", "10s") // last relay/jump report should be more recent than this

		viper.SetDefault("health_log_every", "1h")
		viper.SetDefault("health_events", "100") // max number of health events logged per experiment
		viper.SetDefault("health_startup", "1m") // don't record health history until startup period is over

		viper.SetDefault("host_book", "") // "" so we can check it's been provided
		viper.SetDefault("host_jump", "")
		viper.SetDefault("host_relay", "")

		viper.SetDefault("log_file", "/var/log/status/status.log")
		viper.SetDefault("log_format", "json")
		viper.SetDefault("log_level", "warn")

		viper.SetDefault("port_profile", 6061)
		viper.SetDefault("port_serve", 3007)
		viper.SetDefault("profile", "false")

		viper.SetDefault("query_book_every", "15m")
		viper.SetDefault("reconnect_jump_every", "1d")
		viper.SetDefault("reconnect_relay_every", "1d")

		viper.SetDefault("scheme_book", "https")
		viper.SetDefault("scheme_jump", "https")
		viper.SetDefault("scheme_relay", "https")

		viper.SetDefault("secret_book", "") // "" so we can check it's been provided
		viper.SetDefault("secret_jump", "")
		viper.SetDefault("secret_relay", "")

		viper.SetDefault("timeout_book", "5s")

		basepathBook := viper.GetString("basepath_book")
		basepathJump := viper.GetString("basepath_jump")
		basepathRelay := viper.GetString("basepath_relay")

		emailAuthType := viper.GetString("email_auth_type")
		emailCc := viper.GetStringSlice("email_cc")
		emailFrom := viper.GetString("email_from")
		emailHost := viper.GetString("email_host")
		emailLink := viper.GetString("email_link")
		emailPassword := viper.GetString("email_password")
		emailPort := viper.GetInt("email_port")
		emailSubject := viper.GetString("email_subject")
		emailTo := viper.GetStringSlice("email_to")

		healthLogEveryStr := viper.GetString("health_log_every")
		healthLastActiveStr := viper.GetString("health_last_active")
		healthLastCheckedStr := viper.GetString("health_last_checked")
		healthStartupStr := viper.GetString("health_startup")
		healthEvents := viper.GetInt("health_events")

		hostBook := viper.GetString("host_book")
		hostJump := viper.GetString("host_jump")
		hostRelay := viper.GetString("host_relay")

		logFile := viper.GetString("log_file")
		logFormat := viper.GetString("log_format")
		logLevel := viper.GetString("log_level")

		portProfile := viper.GetInt("port_profile")
		portServe := viper.GetInt("port_serve")

		profile := viper.GetBool("profile")

		queryBookEveryStr := viper.GetString("query_book_every")
		reconnectJumpEveryStr := viper.GetString("reconnect_jump_every")
		reconnectRelayEveryStr := viper.GetString("reconnect_relay_every")

		secretBook := viper.GetString("secret_book")
		secretJump := viper.GetString("secret_jump")
		secretRelay := viper.GetString("secret_relay")

		schemeBook := viper.GetString("scheme_book")
		schemeJump := viper.GetString("scheme_jump")
		schemeRelay := viper.GetString("scheme_relay")

		timeoutBookStr := viper.GetString("timeout_book")

		// Sanity checks
		ok := true

		if secretBook == "" {
			fmt.Println("You must set STATUS_SECRET_BOOK")
			ok = false
		}
		if secretJump == "" {
			fmt.Println("You must set STATUS_SECRET_JUMP")
			ok = false
		}
		if secretRelay == "" {
			fmt.Println("You must set STATUS_SECRET_RELAY")
			ok = false
		}

		if hostBook == "" {
			fmt.Println("You must set STATUS_HOST_BOOK")
			ok = false
		}
		if hostJump == "" {
			fmt.Println("You must set STATUS_HOST_JUMP")
			ok = false
		}
		if hostRelay == "" {
			fmt.Println("You must set STATUS_HOST_RELAY")
			ok = false
		}

		if !ok {
			os.Exit(1)
		}

		sendEmail := true

		if emailFrom == "" {
			fmt.Println("To send email, you must set STATUS_EMAIL_FROM. No emails will be sent.")
			sendEmail = false
		}
		if emailHost == "" {
			fmt.Println("To send email, you must set STATUS_EMAIL_HOST. No emails will be sent.")
			sendEmail = false
		}
		if emailLink == "" {
			fmt.Println("To send email, you must set STATUS_EMAIL_LINK. No emails will be sent.")
			sendEmail = false
		}

		if emailPassword == "" {
			fmt.Println("To send email, you must set STATUS_EMAIL_PASSWORD. No emails will be sent.")
			sendEmail = false
		}
		if emailSubject == "" {
			fmt.Println("To send email, you must set STATUS_EMAIL_SUBJECT. No emails will be sent.")
			sendEmail = false
		}
		if len(emailTo) == 0 {
			fmt.Println("To send email, you must set STATUS_EMAIL_TO. No emails will be sent.")
			sendEmail = false
		}

		// Parse durations
		healthLastActive, err := time.ParseDuration(healthLastActiveStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_HEALTH_LAST=" + healthLastActiveStr)
			os.Exit(1)
		}
		healthLastChecked, err := time.ParseDuration(healthLastCheckedStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_HEALTH_LAST=" + healthLastCheckedStr)
			os.Exit(1)
		}

		healthLogEvery, err := time.ParseDuration(healthLogEveryStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_HEALTH_LOG_EVERY=" + healthLogEveryStr)
			os.Exit(1)
		}

		healthStartup, err := time.ParseDuration(healthStartupStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_HEALTH_STARTUP=" + healthStartupStr)
			os.Exit(1)
		}

		queryBookEvery, err := time.ParseDuration(queryBookEveryStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_QUERY_BOOK_EVERY=" + queryBookEveryStr)
			os.Exit(1)
		}

		reconnectJumpEvery, err := time.ParseDuration(reconnectJumpEveryStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_RECONNECT_JUMP_EVERY=" + reconnectJumpEveryStr)
			os.Exit(1)
		}

		reconnectRelayEvery, err := time.ParseDuration(reconnectRelayEveryStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_RECONNECT_RELAY_EVERY=" + reconnectRelayEveryStr)
			os.Exit(1)
		}

		timeoutBook, err := time.ParseDuration(timeoutBookStr)

		if err != nil {
			fmt.Print("cannot parse duration in STATUS_TIMEOUT_BOOK=" + timeoutBookStr)
			os.Exit(1)
		}

		// set up logging
		switch strings.ToLower(logLevel) {
		case "trace":
			log.SetLevel(log.TraceLevel)
		case "debug":
			log.SetLevel(log.DebugLevel)
		case "info":
			log.SetLevel(log.InfoLevel)
		case "warn":
			log.SetLevel(log.WarnLevel)
		case "error":
			log.SetLevel(log.ErrorLevel)
		case "fatal":
			log.SetLevel(log.FatalLevel)
		case "panic":
			log.SetLevel(log.PanicLevel)
		default:
			fmt.Println("STATUS_LOG_LEVEL can be trace, debug, info, warn, error, fatal or panic but not " + logLevel)
			os.Exit(1)
		}

		switch strings.ToLower(logFormat) {
		case "json":
			log.SetFormatter(&log.JSONFormatter{})
		case "text":
			log.SetFormatter(&log.TextFormatter{})
		default:
			fmt.Println("STATUS_LOG_FORMAT can be json or text but not " + logLevel)
			os.Exit(1)
		}

		if strings.ToLower(logFile) == "stdout" {

			log.SetOutput(os.Stdout) //

		} else {

			file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				log.SetOutput(file)
			} else {
				log.Infof("Failed to log to %s, logging to default stderr", logFile)
			}
		}

		// Report useful info
		log.Infof("status version: %s", versionString())

		log.Debugf("Basepath  (Book): [%s]", basepathBook)
		log.Debugf("Basepath  (Jump): [%s]", basepathJump)
		log.Debugf("Basepath (Relay): [%s]", basepathRelay)

		log.Infof("Send email? [%t]", sendEmail)
		log.Infof("Email from: [%s]", emailFrom)
		log.Infof("Email host: [%s]", emailHost)
		log.Infof("Email link: [%s]", emailLink)
		log.Infof("Email password: [%s...%s]", emailPassword[:4], emailPassword[len(emailPassword)-4:])

		log.Infof("Email port: [%d", emailPort)
		log.Infof("Email subject: [%s]", emailSubject)
		log.Infof("Email to: [%s]", emailTo)
		log.Infof("Email CC: [%s]", emailCc)
		log.Infof("Health last active: [%s]", healthLastActive)
		log.Infof("Health last checked: [%s]", healthLastChecked)

		log.Infof("Health log every: [%s]", healthLogEvery)
		log.Infof("Health startup: [%s]", healthStartup)
		log.Infof("Health events: [%d]", healthEvents)

		log.Debugf("Host  (Book): [%s]", hostBook)
		log.Debugf("Host  (Jump): [%s]", hostJump)
		log.Debugf("Host (Relay): [%s]", hostRelay)

		log.Infof("Log file: [%s]", logFile)
		log.Infof("Log format: [%s]", logFormat)
		log.Infof("Log level: [%s]", logLevel)

		log.Infof("Port for profile: [%d]", portProfile)
		log.Infof("Port for serve: [%d]", portServe)

		log.Infof("Query book every [%s]", queryBookEvery)
		log.Infof("Reconnect jump every [%s]", reconnectJumpEvery)
		log.Infof("Reconnect relay every [%s]", reconnectRelayEvery)

		log.Infof("Profiling is on: [%t]", profile)

		log.Debugf("Scheme  (Book): [%s]", schemeBook)
		log.Debugf("Scheme  (Jump): [%s]", schemeJump)
		log.Debugf("Scheme (Relay): [%s]", schemeRelay)

		log.Debugf("Secret  (Book): [%s...%s]", secretBook[:4], secretBook[len(secretBook)-4:])
		log.Debugf("Secret  (Jump): [%s...%s]", secretJump[:4], secretJump[len(secretJump)-4:])
		log.Debugf("Secret (Relay): [%s...%s]", secretRelay[:4], secretRelay[len(secretRelay)-4:])

		log.Infof("Timeout book [%s]", timeoutBook)

		// Optionally start the profiling server
		if profile {
			go func() {
				URL := "localhost:" + strconv.Itoa(portProfile)
				err := http.ListenAndServe(URL, nil)
				if err != nil {
					log.Errorf(err.Error())
				}
			}()
		}

		ctx, cancel := context.WithCancel(context.Background())

		c := make(chan os.Signal, 1)

		signal.Notify(c, os.Interrupt)

		go func() {
			for range c {
				cancel()
				os.Exit(0)
			}
		}()

		config := config.Config{
			BasepathBook:        basepathBook,
			BasepathJump:        basepathJump,
			BasepathRelay:       basepathRelay,
			EmailAuthType:       emailAuthType,
			EmailCc:             emailCc,
			EmailFrom:           emailFrom,
			EmailHost:           emailHost,
			EmailLink:           emailLink,
			EmailPassword:       emailPassword,
			EmailPort:           emailPort,
			EmailSubject:        emailSubject,
			EmailTo:             emailTo,
			HealthEvents:        healthEvents,
			HealthLastActive:    healthLastActive,
			HealthLastChecked:   healthLastChecked,
			HealthLogEvery:      healthLogEvery,
			HealthStartup:       healthStartup,
			HostBook:            hostBook,
			HostJump:            hostJump,
			HostRelay:           hostRelay,
			Port:                portServe,
			QueryBookEvery:      queryBookEvery,
			ReconnectJumpEvery:  reconnectJumpEvery,
			ReconnectRelayEvery: reconnectRelayEvery,
			SchemeBook:          schemeBook,
			SchemeJump:          schemeJump,
			SchemeRelay:         schemeRelay,
			SecretBook:          secretBook,
			SecretJump:          secretJump,
			SecretRelay:         secretRelay,
			SendEmail:           sendEmail,
			TimeoutBook:         timeoutBook,
		}

		server.Run(ctx, config)

	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

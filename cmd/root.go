package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

const (
	OUTPUT_PADDING       = 3
	LOGGER_MODULE        = "wp2hugo"
	LOGGER_FORMAT        = "[%{level:.6s}] %{message}"
	LOGGER_FORMAT_COLORS = "%{color}[%{level:.6s}] %{color:reset}%{message}"
)

// global params (flags)
var (
	logLevel string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "wp2hugo",
	Short:   "Wordpress to Hugo converter",
	Long:    ``,
	Version: appVersion,
}

// global instance of logger
var log = logging.MustGetLogger(LOGGER_MODULE)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", "INFO", "Log level (CRITICIAL, ERROR, WARNING, NOTICE, INFO, DEBUG)")
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	viper.SetEnvPrefix("sap_td")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv() // read in environment variables that match

	// configure logging
	var logLevelStr = viper.GetString("log.level")
	// try to convert string log level
	logLevel, err := logging.LogLevel(logLevelStr)
	if err != nil {
		fmt.Printf("Invalid logging level: \"%s\"\n", logLevelStr)
		os.Exit(1)
	}

	loggerFormat := LOGGER_FORMAT
	loggerFormat = LOGGER_FORMAT_COLORS

	formatterStdErr := logging.NewBackendFormatter(
		// out, prefix flag
		logging.NewLogBackend(os.Stderr, "", 0),
		logging.MustStringFormatter(loggerFormat),
	)
	logging.SetBackend(formatterStdErr)
	logging.SetLevel(logLevel, LOGGER_MODULE)
}

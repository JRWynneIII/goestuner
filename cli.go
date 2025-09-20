package main

var cli struct {
	Verbose bool `help:"Prints debug output by default"`
	Probe   struct {
	} `cmd:"" help:"List the available radios and SoapySDR configuration"`
	Tune struct {
		File string `help:"Pass a file of complex values instead of a radio"`
	} `cmd:"" help:"Starts the frontend webserver"`
}

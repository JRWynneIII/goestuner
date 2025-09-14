package main

var cli struct {
	Verbose bool `help:"Prints debug output by default"`
	Tune    struct {
	} `cmd:"" help:"Starts the frontend webserver"`
}

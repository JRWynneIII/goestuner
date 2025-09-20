# GOESTuner

`goestuner` is a simple CLI tool that can be used to help orient your parabolic dish with one of the GOES satellites. Think of it as a simple 'signal strength meter' for GOES/HRIT satellites. 

![Screenshot of goestuner running](docs/tui.png)

### Dependencies
* SoapySDR and any required module (e.g. SoapyRTLTCP for RTL_TCP)
* `libsathelper`: See [here](https://github.com/opensatelliteproject/libsathelper/blob/master/README.md) for build and installation instructions
* `libcorrect`: See above
* Go version 1.18+

### Supported SDRs

So far this tool has only been verified as working with the SDRs listed below, but theoretically, it should work for any SDR that SoapySDR supports, and any SDR that supports complex samples. I do not have the resources to test every SDR under the sun, so if anyone is able to test this with other SDR's supported by SoapySDR, please let me know so they can be added to the list of supported radios!

* RTL-SDR Blog v3
* `rtl_tcp` (via the SoapyRTLTCP module)

### Installation

Once the dependencies are satisfied, you can simply install `goestuner` with `go install`:
```
go install github.com/jrwynneiii/goestuner@latest
```

### Usage
```
Usage: goestuner <command> [flags]

Flags:
  -h, --help       Show context-sensitive help.
      --verbose    Prints debug output by default

Commands:
  probe [flags]
    List the available radios and SoapySDR configuration

  tune [flags]
    Starts the frontend webserver

Run "goestuner <command> --help" for more information on a command.
```

* `probe`: Queries SoapySDR to list the available SDRs and their respctive settings (NOTE: Does not show anything for `rtl_tcp` devices)
* `tune`: Starts the HRIT demodulator/decoder and TUI.

### Acknowledgements:

I'd like to thank the [Open Satellite Project](https://github.com/opensatelliteproject) for creating `libsathelper`, and `SatHelperApp`; these two projects were extremely helpful in the development of the demodulator and decoder, and served, not only as a good reference point for development of `goestuner`, but as a wonderful reference for learning various concepts about SDR and xRIT programming. 

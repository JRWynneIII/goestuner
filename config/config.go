package config

import "github.com/jrwynneiii/goestuner/radio"

func AutoConfig() {
	if _, err := radio.InitSoapySDR(); err == nil {
	} else {
		panic(err)
	}

}

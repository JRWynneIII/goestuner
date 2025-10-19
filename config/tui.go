package config

import (
	"regexp"
	"slices"

	"github.com/jrwynneiii/goestuner/radio"
	"github.com/rivo/tview"
)

func StartConfigTUI(subsys *radio.SoapySubsystem, path string) {
	app := tview.NewApplication()
	form := tview.NewForm()

	availDrivers := []string{""}
	for _, dev := range subsys.Devices {
		if !slices.Contains(availDrivers, dev.Driver) {
			availDrivers = append(availDrivers, dev.Driver)
		}
	}

	re := regexp.MustCompile(`^.*librtltcpSupport.so$`)
	for _, m := range subsys.Modules {
		if re.MatchString(m.Name) {
			availDrivers = append(availDrivers, "rtltcp")
			break
		}
	}

	form.AddDropDown("Driver:", availDrivers, 0, func(selected string, idx int) {
		if dd := form.GetFormItemByLabel("Device:"); dd != nil {
			if selected != "rtltcp" {
				availDevices := subsys.GetDevicesByDriver(selected)
				strDevices := []string{}
				for _, dev := range availDevices {
					strDevices = append(strDevices, dev.GetHardwareKey())
				}
				dd.(*tview.DropDown).SetOptions(strDevices, nil)
			} else {
				form.RemoveFormItem(form.GetFormItemIndex("Device:"))
				form.AddInputField("Address:", "127.0.0.1", 20, nil, nil)
				form.AddInputField("Port:", "1234", 20, nil, nil)
			}
		}
	})

	form.AddDropDown("Device:", []string{}, 0, nil)

	form.AddInputField("Gain:", "5", 20, nil, nil)
	form.AddInputField("Default Frequency (kHz):", "1694100000", 20, nil, nil)
	form.AddInputField("Sample Rate:", "2048000", 20, nil, nil)
	form.AddButton("Generate config file", func() {
		app.Suspend(func() {
			_, driver := form.GetFormItemByLabel("Driver:").(*tview.DropDown).GetCurrentOption()
			var device string
			var addr string
			var port string
			if driver == "rtltcp" {
				addr = form.GetFormItemByLabel("Address:").(*tview.InputField).GetText()
				port = form.GetFormItemByLabel("Port:").(*tview.InputField).GetText()
			} else {
				_, device = form.GetFormItemByLabel("Device:").(*tview.DropDown).GetCurrentOption()
			}
			gain := form.GetFormItemByLabel("Gain:").(*tview.InputField).GetText()
			freq := form.GetFormItemByLabel("Default Frequency (kHz):").(*tview.InputField).GetText()
			srate := form.GetFormItemByLabel("Sample Rate:").(*tview.InputField).GetText()

			GenerateConfigFile(path, driver, device, addr, port, gain, freq, srate)

		})
		app.Stop()
	})
	form.AddButton("Exit without saving", func() {
		app.Stop()
	})

	form.SetBorder(true)
	form.SetTitle("Configuration")

	if err := app.SetRoot(form, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

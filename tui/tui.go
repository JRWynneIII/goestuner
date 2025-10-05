package tui

import (
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gdamore/tcell/v2"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/datalink"
	"github.com/jrwynneiii/goestuner/demod"
	"github.com/jrwynneiii/goestuner/radio"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
)

// Making this global so that all modules can set the log output to this io.Writer
var LogOut *tview.TextView
var DebugOut *tview.TextView

func StartUI(decoder *datalink.Decoder, demodulator *demod.Demodulator, r *radio.Radio[complex64], enableFFT bool, tuiConf config.TuiConf) {
	enableDebugOutput := false
	debugVisible := false
	pause := false
	app := tview.NewApplication()

	LogOut = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	var logMutex sync.Mutex
	var debugMutex sync.Mutex
	LogOut.SetChangedFunc(func() {
		if !pause {
			logMutex.Lock()
			LogOut.ScrollToEnd()
			app.Draw()
			logMutex.Unlock()
		}
	})

	LogOut.SetBorder(true).SetTitle("Log Output")
	log.SetOutput(LogOut)

	DebugOut = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	DebugOut.SetChangedFunc(func() {
		if !pause {
			debugMutex.Lock()
			DebugOut.ScrollToEnd()
			app.Draw()
			debugMutex.Unlock()
		}
	})
	DebugOut.SetBorder(true).SetTitle("Debug")

	// Init our tables
	channelData := &ChannelTableData{}
	lockData := &LockTableData{}
	channelStats := tview.NewTable().SetContent(channelData)
	lockTable := tview.NewTable().SetContent(lockData)
	channelStats.SetSelectable(false, false).SetBorder(true).SetTitle("Per-Channel Stats")
	lockTable.SetSelectable(false, false).SetBorder(false)

	// Init the FFT plot
	signalPlot := tvxwidgets.NewPlot()
	signalPlot.SetLineColor([]tcell.Color{tcell.ColorLightSkyBlue})
	signalPlot.SetMarker(tvxwidgets.PlotMarkerBraille)
	signalPlot.SetBorder(true)
	signalPlot.SetTitle("Signal")

	// Init our main gauge's that are used for signal strength, etc
	signalGauge := tvxwidgets.NewUtilModeGauge()
	signalGauge.SetLabel("Signal Strength:             ")
	signalGauge.SetLabelColor(tcell.ColorLightSkyBlue)
	signalGauge.SetWarnPercentage(99)
	signalGauge.SetCritPercentage(100)
	signalGauge.SetEmptyColor(tcell.ColorBlack)
	signalGauge.SetBorder(false)

	berGauge := tvxwidgets.NewUtilModeGauge()
	berGauge.SetLabel("Viterbi Error Rate:          ")
	berGauge.SetLabelColor(tcell.ColorLightSkyBlue)
	berGauge.SetWarnPercentage(tuiConf.VitWarnPct)
	berGauge.SetCritPercentage(tuiConf.VitCritPct)
	berGauge.SetEmptyColor(tcell.ColorBlack)
	berGauge.SetBorder(false)

	rsCorrectionsGauge := tvxwidgets.NewUtilModeGauge()
	rsCorrectionsGauge.SetLabel("Reed-Soloman Corrections:    ")
	rsCorrectionsGauge.SetLabelColor(tcell.ColorLightSkyBlue)
	rsCorrectionsGauge.SetWarnPercentage(tuiConf.RsWarnPct)
	rsCorrectionsGauge.SetCritPercentage(tuiConf.RsCritPct)
	rsCorrectionsGauge.SetEmptyColor(tcell.ColorBlack)
	rsCorrectionsGauge.SetBorder(false)

	// Init our decoder stats flex rows
	gaugeBox := tview.NewFlex()
	gaugeBox.SetDirection(tview.FlexRow)
	gaugeBox.AddItem(signalGauge, 0, 1, false)
	gaugeBox.AddItem(berGauge, 0, 1, false)
	gaugeBox.AddItem(rsCorrectionsGauge, 0, 1, false)
	gaugeBox.SetTitle("Signal Stats")
	gaugeBox.SetBorder(true)

	decoderStats := tview.NewFlex().SetDirection(tview.FlexRow)
	decoderStats.AddItem(tview.NewBox(), 0, 1, false)
	decoderStats.AddItem(lockTable, 0, 2, false)
	decoderStats.AddItem(tview.NewBox(), 0, 1, false)
	decoderStats.SetBorder(true)
	decoderStats.SetTitle("Decoder Status")

	// Init our page and columns
	page := tview.NewFlex().SetDirection(tview.FlexColumn)

	leftCol := tview.NewFlex().SetDirection(tview.FlexRow)
	leftCol.AddItem(channelStats, 0, 6, false)
	leftCol.AddItem(decoderStats, 0, 1, false)
	if enableFFT {
		leftCol.AddItem(signalPlot, 0, 2, false)
	}

	rightCol := tview.NewFlex().SetDirection(tview.FlexRow)
	rightCol.AddItem(gaugeBox, 0, 4, false)
	if tuiConf.EnableLogOutput {
		rightCol.AddItem(LogOut, 0, 2, false)
	}
	if enableDebugOutput {
		rightCol.AddItem(DebugOut, 0, 2, false)
	}
	page.AddItem(leftCol, 0, 2, false)
	page.AddItem(rightCol, 0, 5, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			app.Stop()
		case 'f':
			//Pause radio
			app.Suspend(func() {
				// Reset to stdout log output and Pause the radio
				log.SetOutput(os.Stdout)
				log.Debugf("Pausing SDR")
				r.Pause()
				//Wait for physical layer to drain
				log.Debug("Waiting for phyiscal layer to drain")
				for len(demodulator.SampleInput) > 0 {
					time.Sleep(50 * time.Millisecond)
				}
				//Forcibly flush the datalink layer. This sucks but it is what it is
				log.Debug("Flushing datalink layer")
				for len(*demodulator.SymbolsOutput) > 0 {
					select {
					case c := <-*demodulator.SymbolsOutput:
						func(a any) {}(c)
					}
				}
				//Reset stats
				log.Debug("Resetting channel and decoder stats")
				ResetChannelAndDecoderStats()

				//Reset datalink layer
				log.Debug("Resetting Lock and guage stats")
				decoder.FrameLock = false
				decoder.SigQuality = 0.0
				decoder.AverageRsCorrections = 0
				decoder.RxPacketsPerChannel = make(map[int]int)
				decoder.DroppedPacketsPerChannel = make(map[int]int)
				decoder.TotalFramesProcessed = 0
				signalGauge.SetValue(float64(decoder.SigQuality))
				berGauge.SetValue(0.0)
				rsCorrectionsGauge.SetValue(float64(decoder.AverageRsCorrections))

				//Restart radio
				log.Debug("Reconnecting to SDR...")
				r.Connect()
				log.Debug("Flushed!")
				log.SetOutput(LogOut)
			})
		case 'd':
			if !enableDebugOutput {
				enableDebugOutput = true
			} else {
				enableDebugOutput = false
			}
		case 'p':
			if pause {
				pause = false
			} else {
				pause = true
			}
		}
		return event
	})
	//Update all data in our UI.
	go func() {
		for {
			if !pause {
				// Gather stats from decoder
				decoder.StatsMutex.RLock()
				frameLock := decoder.FrameLock
				ber := decoder.Viterbi.GetPercentBER()
				sigquality := decoder.SigQuality
				RSCorrectionPercent := decoder.AverageRsCorrections
				packetsPerChannel := decoder.RxPacketsPerChannel
				droppedPacketsPerChannel := decoder.DroppedPacketsPerChannel
				totalFrames := decoder.TotalFramesProcessed

				// Update channel stats
				var totalPacketsDropped int
				for idx := 0; idx < len(channels); idx++ {
					channel := ReadChannelData(idx)
					channel.NumPackets = packetsPerChannel[channel.ID]
					channel.NumPacketsDropped = droppedPacketsPerChannel[channel.ID]
					WriteChannelData(idx, channel)
					totalPacketsDropped += channel.NumPacketsDropped
				}
				decoder.StatsMutex.RUnlock()
				//Update gauges
				signalGauge.SetValue(float64(sigquality))
				berGauge.SetValue(float64(ber))
				rsCorrectionsGauge.SetValue(float64(RSCorrectionPercent))

				//Update decoder stats
				WriteOverallDecoderStats(DecoderStats{
					FrameLock:           frameLock,
					TotalPackets:        totalFrames,
					TotalDroppedPackets: totalPacketsDropped,
				})

				//Update signal plot
				demodulator.FFTMutex.RLock()
				fft := demodulator.CurrentFFT
				demodulator.FFTMutex.RUnlock()

				if len(fft) > 0 {
					var bins []float64
					for _, val := range fft {
						bins = append(bins, val)
					}
					signalPlot.SetData([][]float64{bins})
				}

				if enableDebugOutput && !debugVisible {
					rightCol.AddItem(DebugOut, 0, 2, false)
					debugVisible = true
				} else if !enableDebugOutput && debugVisible {
					rightCol.RemoveItem(DebugOut)
					debugVisible = false
				}

				app.Draw()
			}
			//Sleep half a second
			time.Sleep(time.Duration(tuiConf.RefreshMs) * time.Millisecond)

		}
	}()

	// Start the TUI
	if err := app.SetRoot(page, true).EnableMouse(true).Run(); err != nil {
		log.Fatalf("Could not start UI: %v", err)
	}
}

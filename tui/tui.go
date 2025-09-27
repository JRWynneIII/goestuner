package tui

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/gdamore/tcell/v2"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/decode"
	"github.com/jrwynneiii/goestuner/demod"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
)

// Making this global so that all modules can set the log output to this io.Writer
var LogOut *tview.TextView

func StartUI(decoder *decode.Decoder, demodulator *demod.Demodulator, enableFFT bool, tuiConf config.TuiConf) {
	app := tview.NewApplication()

	LogOut = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	LogOut.SetChangedFunc(func() {
		LogOut.ScrollToEnd()
		app.Draw()
	})
	LogOut.SetBorder(true).SetTitle("Log Output")
	log.SetOutput(LogOut)

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
	page.AddItem(leftCol, 0, 2, false)
	page.AddItem(rightCol, 0, 5, false)

	//Update all data in our UI.
	go func() {
		for {
			// Gather stats from decoder
			frameLock := decoder.FrameLock
			ber := decoder.Viterbi.GetPercentBER()
			sigquality := decoder.SigQuality
			averageRsCorrections := decoder.AverageRsCorrections
			packetsPerChannel := decoder.RxPacketsPerChannel
			droppedPacketsPerChannel := decoder.DroppedPacketsPerChannel
			totalFrames := decoder.TotalFramesProcessed
			RSCorrectionPercent := (averageRsCorrections / float32(totalFrames)) * 100

			// Update channel stats
			var totalPacketsDropped int
			for idx, channel := range channels {
				channel.NumPackets = packetsPerChannel[channel.ID]
				channel.NumPacketsDropped = droppedPacketsPerChannel[channel.ID]
				channels[idx] = channel
				totalPacketsDropped += channel.NumPacketsDropped
			}
			//Update gauges
			signalGauge.SetValue(float64(sigquality))
			berGauge.SetValue(float64(ber))
			rsCorrectionsGauge.SetValue(float64(RSCorrectionPercent))

			//Update decoder stats
			overallDecoderStats.FrameLock = frameLock
			overallDecoderStats.TotalPackets = totalFrames
			overallDecoderStats.TotalDroppedPackets = totalPacketsDropped

			//Update signal plot
			if len(demodulator.CurrentFFT) > 0 {
				var bins []float64
				for _, val := range demodulator.CurrentFFT {
					bins = append(bins, val)
				}
				signalPlot.SetData([][]float64{bins})
			}

			app.Draw()
			//Sleep half a second
			//TODO: maybe make this a config value, or ability to set on the fly?
			time.Sleep(time.Duration(tuiConf.RefreshMs) * time.Millisecond)

		}
	}()

	// Start the TUI
	if err := app.SetRoot(page, true).EnableMouse(true).Run(); err != nil {
		log.Fatalf("Could not start UI: %v", err)
	}
}

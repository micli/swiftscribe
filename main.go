package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	uuid "github.com/satori/go.uuid"
	"github.com/schollz/progressbar/v3"
)

var (
	Version = "0.1.0"
)

// This function used for chunk big WAV file into 1 minute for each.
// The input parameter is the WAV file decoder and the UUID as name of temporary folder.
// First, it need to get the duration of the big WAV file and calculate how many small WAV file are in temporary folder.
// Then, it will read the big WAV file and write the small WAV file.
// Sample rate of the big WAV file will be used for the small WAV file.
// The small WAV file will be saved in temporary folder with the name of index like: 01.wav .
func chunkWAVfile(filename string, uuid string) error {
	// Get duration of the big WAV file.
	wavefile, err := os.Open(filename)
	if err != nil {
		return err
	}
	decoder := wav.NewDecoder(wavefile)
	defer wavefile.Close()

	decoder.ReadInfo()
	duration, err := decoder.Duration()
	if err != nil {
		return err
	}
	// Calculate how many small WAV file are in temporary folder.
	chunkCount := int(duration.Minutes())
	if duration.Minutes() > float64(chunkCount) {
		chunkCount++
	}

	bar := progressbar.NewOptions(chunkCount,
		progressbar.OptionSetDescription("Chunk audio in pieces..."),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(15),
	)

	bufferSize := int(decoder.SampleRate) * 60 * int(decoder.BitDepth) / 8 // 1 minute buffer size.
	minuteWaveFileBuffer := &audio.IntBuffer{Data: make([]int, bufferSize), Format: decoder.Format()}
	// Read the big WAV file and write the small WAV file.
	for i := 0; i < chunkCount; i++ {
		// Create small WAV file.
		chunkFilename := fmt.Sprintf("./%s/%d.wav", uuid, i)
		chunkFile, err := os.Create(chunkFilename)
		if err != nil {
			return err
		}
		defer chunkFile.Close()
		chunkEncoder := wav.NewEncoder(chunkFile, int(decoder.SampleRate), int(decoder.BitDepth), int(decoder.NumChans), int(decoder.WavAudioFormat))
		// Read the big WAV file and write the small WAV file.
		_, err = decoder.PCMBuffer(minuteWaveFileBuffer)
		if err != nil {
			break
		}
		chunkEncoder.Write(minuteWaveFileBuffer)
		// clean minuteWaveFileBuffer set to 0.
		copy(minuteWaveFileBuffer.Data, make([]int, bufferSize))
		chunkEncoder.Close()
		bar.Add(1)
	}
	return nil
}

func main() {
	start := time.Now()
	var (
		serviceURL string
		key        string
		language   string
	)

	flag.StringVar(&serviceURL, "service", "", "backend service URL")
	flag.StringVar(&serviceURL, "s", "", "backend service URL")
	flag.StringVar(&key, "key", "", "access key")
	flag.StringVar(&key, "k", "", "access key")
	flag.StringVar(&language, "language", "en-US", "language of audio file")
	flag.StringVar(&language, "lang", "en-US", "language of audio file")
	flag.Parse()
	// Check input parameters.
	if serviceURL == "" || key == "" || len(flag.Args()) < 1 {
		fmt.Println("How to use: -s <service_url> -k <key> [-lang <language>] <filename>")
		os.Exit(1)
	}

	filename := flag.Args()[0]
	uuid := uuid.NewV4()
	os.Mkdir(uuid.String(), os.ModePerm)
	// Chunk big WAV file into 1 minute for each.
	chunkWAVfile(filename, uuid.String())
	println("Starting audio files transcribe.")
	// Call Azure Speech to Text API to transcribe audio files.
	azurespeech := NewAzureFastTranscription(serviceURL, key, language)
	transcriptionText, err := azurespeech.RunTranscribeAsync(uuid.String())
	if err != nil {
		fmt.Println("Error:", err)
	}
	// Write transcription result to file.
	transFilename := strings.Replace(filename, ".wav", ".txt", len(filename)-4)
	os.WriteFile(transFilename, []byte(transcriptionText), 0644)
	// clear temporary folder.
	os.RemoveAll(uuid.String())
	println(fmt.Sprintf("Execution time: %02d:%02d:%02d.%03d",
		int(time.Since(start).Hours()),
		int(time.Since(start).Minutes())%60,
		int(time.Since(start).Seconds())%60,
		time.Since(start).Milliseconds()%1000))
	println("Completed.")
}

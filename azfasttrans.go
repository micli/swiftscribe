// Description: This file is the implementation of Azure speech fast transcription.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

type AzureFastTranscription struct {
	Endpoint        string
	SubscriptionKey string
	Language        string
	Maxconcurrency  int
}

// 简化的返回结构体
type transcribeResult struct {
	CombinedPhrases []struct {
		Text string `json:"text"`
	} `json:"combinedPhrases"`
}

// NewAzureFastTranscription 创建一个新的 AzureFastTranscription 实例
func NewAzureFastTranscription(endpoint, subscriptionKey, language string) *AzureFastTranscription {
	return &AzureFastTranscription{
		Endpoint:        endpoint,
		SubscriptionKey: subscriptionKey,
		Language:        language,
		Maxconcurrency:  16,
	}
}

// GetTranscription 发起请求并解析结果
func (a *AzureFastTranscription) GetTranscription(audioFilePath, language string) (string, error) {
	fileData, err := os.ReadFile(audioFilePath)
	if err != nil {
		return "", err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	audioPart, err := writer.CreateFormFile("audio", audioFilePath)
	if err != nil {
		return "", err
	}
	if _, err = audioPart.Write(fileData); err != nil {
		return "", err
	}

	definition := fmt.Sprintf(`{"locales":["%s"]}`, language)
	if err = writer.WriteField("definition", definition); err != nil {
		return "", err
	}
	writer.Close()

	url := fmt.Sprintf("%s/speechtotext/transcriptions:transcribe?api-version=2024-11-15", a.Endpoint)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Ocp-Apim-Subscription-Key", a.SubscriptionKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("服务响应异常: %d", resp.StatusCode)
	}

	var result transcribeResult
	respBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", err
	}

	var transcribedText string
	for _, phrase := range result.CombinedPhrases {
		transcribedText += phrase.Text + " "
	}
	return transcribedText, nil
}

func (a *AzureFastTranscription) RunTranscribeAsync(folder string) (string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "", fmt.Errorf("Cannot read folder: %v", err)
	}

	var wavFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".wav") {
			wavFiles = append(wavFiles, filepath.Join(folder, entry.Name()))
		}
	}

	// Sort files by name
	sort.Strings(wavFiles)

	bar := progressbar.NewOptions(len(wavFiles),
		progressbar.OptionSetDescription("Transcribing audio..."),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(15),
	)

	var wg sync.WaitGroup
	sem := make(chan struct{}, a.Maxconcurrency)
	results := make([]string, len(wavFiles))
	errors := make(chan error, len(wavFiles))

	for i, file := range wavFiles {
		wg.Add(1)
		go func(i int, file string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			transcription, err := a.GetTranscription(file, a.Language)
			if err != nil {
				errors <- err
				return
			}
			results[i] = transcription
			// println("Transcribe -- index:", i, "transcription:", transcription)
			bar.Add(1)
		}(i, file)
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		return "", <-errors
	}

	return strings.Join(results, " "), nil
}

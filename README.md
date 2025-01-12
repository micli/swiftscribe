# swiftscribe

A tool that leverages Azure Fast Transcription to accelerate the transcription of audio files. This allows 40 minutes of audio to be transcribed in just 2 minutes.


## How to use?
```bash
./swiftscribe -s <Azure Speech Endpoint url> -k <Azure Speech Service Key> -lang <language code, en-US...> <Wave audio file path>
```

Transcribe result will be located at same path of wave file.


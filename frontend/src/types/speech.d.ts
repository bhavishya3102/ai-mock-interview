// Browser SpeechRecognition is non-standard and missing in default lib.dom typings.
// We don't use it directly (the speech-to-text library wraps it), but we declare
// the constructors here for any lazy globals reference paths.
declare global {
  interface Window {
    webkitSpeechRecognition?: unknown;
    SpeechRecognition?: unknown;
  }
}

export {};

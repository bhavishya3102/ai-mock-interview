import { useCallback, useEffect, useRef, useState } from "react";

export interface UseAudioRecorderReturn {
  isRecording: boolean;
  /** Seconds since the current recording started; 0 when idle. */
  elapsedSeconds: number;
  /** True if the browser exposes MediaRecorder + a usable mime type. */
  isSupported: boolean;
  error: string | null;
  start: () => Promise<void>;
  /** Stops recording and resolves with the captured Blob (or null if never started). */
  stop: () => Promise<Blob | null>;
}

// Picked in preference order: webm/opus is widest (Chrome, Firefox, Edge);
// ogg/opus covers older Firefox; mp4/aac is Safari's only native option.
const PREFERRED_MIME_TYPES = [
  "audio/webm;codecs=opus",
  "audio/webm",
  "audio/ogg;codecs=opus",
  "audio/ogg",
  "audio/mp4",
  "audio/mpeg",
];

function pickMimeType(): string | null {
  if (typeof MediaRecorder === "undefined") return null;
  for (const mime of PREFERRED_MIME_TYPES) {
    if (MediaRecorder.isTypeSupported(mime)) return mime;
  }
  return null;
}

export function isMediaRecorderSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof MediaRecorder !== "undefined" &&
    typeof navigator !== "undefined" &&
    !!navigator.mediaDevices?.getUserMedia &&
    pickMimeType() !== null
  );
}

/**
 * useAudioRecorder wraps the MediaRecorder API. Unlike Web Speech, this works
 * in Firefox, Safari, mobile browsers, and Chromium without dropping words.
 *
 * The caller gets a Blob on stop() — it's their job to upload/transcribe it.
 */
export function useAudioRecorder(): UseAudioRecorderReturn {
  const [isRecording, setIsRecording] = useState(false);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const [error, setError] = useState<string | null>(null);

  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const streamRef = useRef<MediaStream | null>(null);
  const tickRef = useRef<number | null>(null);
  const stopResolverRef = useRef<((blob: Blob | null) => void) | null>(null);

  const cleanupStream = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((t) => t.stop());
      streamRef.current = null;
    }
    if (tickRef.current !== null) {
      window.clearInterval(tickRef.current);
      tickRef.current = null;
    }
  }, []);

  const start = useCallback(async () => {
    setError(null);
    if (recorderRef.current) return;
    if (!isMediaRecorderSupported()) {
      setError("Audio recording is not supported in this browser.");
      return;
    }
    const mimeType = pickMimeType();
    if (!mimeType) {
      setError("No supported audio format available in this browser.");
      return;
    }

    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch (err) {
      const name = err instanceof Error ? err.name : "";
      if (name === "NotAllowedError" || name === "SecurityError") {
        setError("Microphone access was denied. Allow it in your browser settings and try again.");
      } else if (name === "NotFoundError" || name === "OverconstrainedError") {
        setError("No microphone was detected. Plug one in and try again.");
      } else {
        setError("Could not start the microphone.");
      }
      return;
    }

    streamRef.current = stream;
    chunksRef.current = [];

    let recorder: MediaRecorder;
    try {
      recorder = new MediaRecorder(stream, { mimeType });
    } catch {
      cleanupStream();
      setError("Could not start the audio recorder.");
      return;
    }

    recorder.ondataavailable = (event) => {
      if (event.data && event.data.size > 0) {
        chunksRef.current.push(event.data);
      }
    };

    recorder.onerror = () => {
      setError("The microphone stream failed mid-recording.");
    };

    recorder.onstop = () => {
      const blob =
        chunksRef.current.length > 0
          ? new Blob(chunksRef.current, { type: mimeType })
          : null;
      chunksRef.current = [];
      recorderRef.current = null;
      cleanupStream();
      setIsRecording(false);
      setElapsedSeconds(0);
      const resolver = stopResolverRef.current;
      stopResolverRef.current = null;
      if (resolver) resolver(blob);
    };

    // 250ms timeslice ensures dataavailable fires periodically — without it
    // some browsers buffer everything until stop() and a tab-crash loses all
    // audio.
    recorder.start(250);
    recorderRef.current = recorder;
    setIsRecording(true);

    const startedAt = Date.now();
    setElapsedSeconds(0);
    tickRef.current = window.setInterval(() => {
      setElapsedSeconds(Math.floor((Date.now() - startedAt) / 1000));
    }, 250);
  }, [cleanupStream]);

  const stop = useCallback((): Promise<Blob | null> => {
    const recorder = recorderRef.current;
    if (!recorder || recorder.state === "inactive") {
      return Promise.resolve(null);
    }
    return new Promise<Blob | null>((resolve) => {
      stopResolverRef.current = resolve;
      try {
        recorder.stop();
      } catch {
        stopResolverRef.current = null;
        cleanupStream();
        setIsRecording(false);
        setElapsedSeconds(0);
        recorderRef.current = null;
        resolve(null);
      }
    });
  }, [cleanupStream]);

  // Belt-and-braces: never leak a mic indicator if the component unmounts
  // mid-recording (route change, navigation away from the question).
  useEffect(() => {
    return () => {
      const recorder = recorderRef.current;
      if (recorder && recorder.state !== "inactive") {
        try {
          recorder.stop();
        } catch {
          // ignored — we're tearing down anyway
        }
      }
      cleanupStream();
    };
  }, [cleanupStream]);

  return {
    isRecording,
    elapsedSeconds,
    isSupported: isMediaRecorderSupported(),
    error,
    start,
    stop,
  };
}

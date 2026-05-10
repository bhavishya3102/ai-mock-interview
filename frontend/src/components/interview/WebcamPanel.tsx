import type { ReactElement } from "react";
import Webcam from "react-webcam";
import { Camera, CameraOff, MonitorOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useWebcamPermission } from "@/hooks/useWebcamPermission";

interface WebcamPanelProps {
  className?: string;
}

export function WebcamPanel({ className }: WebcamPanelProps): ReactElement {
  const { state, request, onUserMedia, onUserMediaError } = useWebcamPermission();

  return (
    <div
      className={`relative flex aspect-video w-full items-center justify-center overflow-hidden rounded-2xl border bg-secondary/50 ${className ?? ""}`}
    >
      {state === "unsupported" ? (
        <div className="flex flex-col items-center gap-2 px-6 text-center text-sm text-muted-foreground">
          <MonitorOff className="h-8 w-8" />
          Camera not available in this browser.
        </div>
      ) : null}

      {state === "denied" ? (
        <div className="flex flex-col items-center gap-3 px-6 text-center text-sm text-muted-foreground">
          <CameraOff className="h-8 w-8" />
          Camera access blocked. Allow it in your browser to enable the preview.
          <Button variant="outline" size="sm" onClick={() => void request()}>
            Try again
          </Button>
        </div>
      ) : null}

      {state === "idle" ? (
        <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 bg-background/80 px-6 text-center text-sm text-muted-foreground backdrop-blur-sm">
          <Camera className="h-8 w-8" />
          <p>Enable your webcam for the practice flow.</p>
          <Button size="sm" onClick={() => void request()}>
            Enable webcam
          </Button>
        </div>
      ) : null}

      {state !== "unsupported" ? (
        <Webcam
          audio={false}
          mirrored
          onUserMedia={onUserMedia}
          onUserMediaError={onUserMediaError}
          className="h-full w-full object-cover"
        />
      ) : null}
    </div>
  );
}

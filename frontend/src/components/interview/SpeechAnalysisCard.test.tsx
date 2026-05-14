import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { SpeechAnalysisCard } from "./SpeechAnalysisCard";

describe("SpeechAnalysisCard", () => {
  it("renders the three delivery metrics", () => {
    render(
      <SpeechAnalysisCard
        analysis={{ fillerCount: 4, wordsPerMinute: 150, longPauseCount: 1 }}
      />,
    );

    const card = screen.getByTestId("speech-analysis");
    expect(within(card).getByText(/filler words/i)).toBeInTheDocument();
    expect(within(card).getByText("4")).toBeInTheDocument();
    expect(within(card).getByText(/words\/min/i)).toBeInTheDocument();
    expect(within(card).getByText("150")).toBeInTheDocument();
    expect(within(card).getByText(/long pauses/i)).toBeInTheDocument();
    expect(within(card).getByText("1")).toBeInTheDocument();
  });

  it("shows a friendly hint for unmeasurable pace (wpm=0)", () => {
    render(
      <SpeechAnalysisCard
        analysis={{ fillerCount: 0, wordsPerMinute: 0, longPauseCount: 0 }}
      />,
    );
    expect(screen.getByText(/couldn't measure pace/i)).toBeInTheDocument();
  });
});

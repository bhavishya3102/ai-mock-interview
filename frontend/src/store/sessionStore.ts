import { create } from "zustand";

interface SessionState {
  mockId: string | null;
  activeIndex: number;
  transcript: string;
  setMockId: (mockId: string | null) => void;
  setActive: (index: number) => void;
  appendTranscript: (chunk: string) => void;
  setTranscript: (value: string) => void;
  reset: () => void;
}

const initialState = {
  mockId: null as string | null,
  activeIndex: 0,
  transcript: "",
};

export const useSessionStore = create<SessionState>((set) => ({
  ...initialState,
  setMockId: (mockId) => set({ mockId }),
  setActive: (index) => set({ activeIndex: index, transcript: "" }),
  appendTranscript: (chunk) =>
    set((state) => ({
      transcript: state.transcript ? `${state.transcript} ${chunk}`.trim() : chunk.trim(),
    })),
  setTranscript: (value) => set({ transcript: value }),
  reset: () => set(initialState),
}));

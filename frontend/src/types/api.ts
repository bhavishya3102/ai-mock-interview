export type ID = string;

export interface Question {
  question: string;
  answer: string;
}

export interface Interview {
  mockId: ID;
  jobPosition: string;
  jobDescription: string;
  yearsExperience: number;
  questions: Question[];
  createdAt: string;
}

export interface InterviewSummary {
  mockId: ID;
  jobPosition: string;
  jobDescription: string;
  yearsExperience: number;
  createdAt: string;
}

export interface UserAnswer {
  questionIndex: number;
  question: string;
  userAnswer: string;
  correctAnswer: string;
  feedback: string;
  rating: number;
  createdAt: string;
}

export interface CreateInterviewInput {
  jobPosition: string;
  jobDescription: string;
  yearsExperience: number;
}

export interface SubmitAnswerInput {
  questionIndex: number;
  userAnswer: string;
}

export interface ListResponse<T> {
  items: T[];
  nextCursor?: string;
}

export interface ApiErrorBody {
  error: { code: string; message: string };
}

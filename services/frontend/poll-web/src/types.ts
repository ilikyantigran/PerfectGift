// Domain types for the poll, normalized from the Gateway's snake_case JSON.
// The backend (poll.v1 proto, serialized via protojson) emits enum values as their
// full names, e.g. "QUESTION_TYPE_TEXT". We normalize those to a small union here so
// the rest of the app never has to know the wire spelling.

export type QuestionType = "text" | "single_choice" | "multi_choice";

export interface Option {
  id: string;
  label: string;
}

export interface Question {
  id: string;
  prompt: string;
  type: QuestionType;
  options: Option[];
  required: boolean;
}

export interface Poll {
  pollId: string;
  title: string;
  questions: Question[];
}

// An answer as sent to the Gateway. Shape matches poll.v1.Answer (snake_case).
export interface Answer {
  question_id: string;
  text?: string;
  choice_ids?: string[];
}

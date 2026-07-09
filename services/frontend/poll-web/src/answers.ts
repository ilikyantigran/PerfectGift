// Pure helpers for turning the form's per-question values into the wire payload and
// for client-side required-field validation. Kept framework-free so they are unit-tested
// directly.

import type { Answer, Question } from "./types";
import type { AnswerValue } from "./QuestionField";

export type ValueMap = Record<string, AnswerValue>;

export function emptyValue(): AnswerValue {
  return { text: "", choiceIds: [] };
}

// Is a single question's value non-empty / satisfied?
export function isAnswered(question: Question, value: AnswerValue | undefined): boolean {
  if (!value) return false;
  if (question.type === "text") return value.text.trim().length > 0;
  return value.choiceIds.length > 0;
}

// Ids of required questions the Subject has not answered yet.
export function findMissingRequired(questions: Question[], values: ValueMap): string[] {
  return questions
    .filter((q) => q.required && !isAnswered(q, values[q.id]))
    .map((q) => q.id);
}

// Build the answers array for POST. Only include questions that were actually answered
// (an empty optional question is simply omitted).
export function buildAnswers(questions: Question[], values: ValueMap): Answer[] {
  const out: Answer[] = [];
  for (const q of questions) {
    const v = values[q.id];
    if (!isAnswered(q, v)) continue;
    if (q.type === "text") {
      out.push({ question_id: q.id, text: v!.text.trim() });
    } else {
      out.push({ question_id: q.id, choice_ids: v!.choiceIds });
    }
  }
  return out;
}

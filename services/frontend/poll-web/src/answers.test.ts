import { describe, expect, it } from "vitest";
import { buildAnswers, findMissingRequired, isAnswered, type ValueMap } from "./answers";
import type { Question } from "./types";

const text: Question = { id: "q1", prompt: "Text", type: "text", options: [], required: true };
const single: Question = {
  id: "q2",
  prompt: "Single",
  type: "single_choice",
  options: [{ id: "a", label: "A" }, { id: "b", label: "B" }],
  required: false,
};
const multi: Question = {
  id: "q3",
  prompt: "Multi",
  type: "multi_choice",
  options: [{ id: "x", label: "X" }, { id: "y", label: "Y" }],
  required: true,
};

describe("isAnswered", () => {
  it("treats whitespace-only text as unanswered", () => {
    expect(isAnswered(text, { text: "   ", choiceIds: [] })).toBe(false);
    expect(isAnswered(text, { text: "hi", choiceIds: [] })).toBe(true);
  });

  it("treats an empty choice set as unanswered", () => {
    expect(isAnswered(single, { text: "", choiceIds: [] })).toBe(false);
    expect(isAnswered(single, { text: "", choiceIds: ["a"] })).toBe(true);
  });
});

describe("findMissingRequired", () => {
  it("reports only unanswered required questions", () => {
    const values: ValueMap = {
      q1: { text: "", choiceIds: [] }, // required text, empty -> missing
      q2: { text: "", choiceIds: [] }, // optional -> ignored
      q3: { text: "", choiceIds: ["x"] }, // required multi, answered
    };
    expect(findMissingRequired([text, single, multi], values)).toEqual(["q1"]);
  });
});

describe("buildAnswers", () => {
  it("emits text and choice answers in wire shape, omitting empties", () => {
    const values: ValueMap = {
      q1: { text: "  blue  ", choiceIds: [] },
      q2: { text: "", choiceIds: [] }, // optional & empty -> omitted
      q3: { text: "", choiceIds: ["x", "y"] },
    };
    expect(buildAnswers([text, single, multi], values)).toEqual([
      { question_id: "q1", text: "blue" },
      { question_id: "q3", choice_ids: ["x", "y"] },
    ]);
  });
});

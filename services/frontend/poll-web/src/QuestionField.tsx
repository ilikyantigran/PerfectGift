import type { Question } from "./types";

// A single answer's value: free text, or a set of selected option ids.
export interface AnswerValue {
  text: string;
  choiceIds: string[];
}

interface Props {
  question: Question;
  value: AnswerValue;
  onChange: (next: AnswerValue) => void;
  invalid: boolean;
}

// Renders one question according to its type. Controlled by the parent.
export function QuestionField({ question, value, onChange, invalid }: Props) {
  const describedBy = invalid ? `${question.id}-error` : undefined;

  return (
    <fieldset className={`question${invalid ? " question--invalid" : ""}`}>
      <legend className="question__prompt">
        {question.prompt}
        {question.required && <span className="question__required" aria-hidden="true"> *</span>}
      </legend>

      {question.type === "text" && (
        <textarea
          className="question__text"
          rows={3}
          value={value.text}
          aria-describedby={describedBy}
          aria-required={question.required}
          onChange={(e) => onChange({ ...value, text: e.target.value })}
        />
      )}

      {question.type === "single_choice" && (
        <div className="question__options" role="radiogroup" aria-describedby={describedBy}>
          {question.options.map((opt) => (
            <label key={opt.id} className="option">
              <input
                type="radio"
                name={question.id}
                value={opt.id}
                checked={value.choiceIds.includes(opt.id)}
                onChange={() => onChange({ ...value, choiceIds: [opt.id] })}
              />
              <span>{opt.label}</span>
            </label>
          ))}
        </div>
      )}

      {question.type === "multi_choice" && (
        <div className="question__options" aria-describedby={describedBy}>
          {question.options.map((opt) => {
            const checked = value.choiceIds.includes(opt.id);
            return (
              <label key={opt.id} className="option">
                <input
                  type="checkbox"
                  name={question.id}
                  value={opt.id}
                  checked={checked}
                  onChange={() => {
                    const next = checked
                      ? value.choiceIds.filter((id) => id !== opt.id)
                      : [...value.choiceIds, opt.id];
                    onChange({ ...value, choiceIds: next });
                  }}
                />
                <span>{opt.label}</span>
              </label>
            );
          })}
        </div>
      )}

      {invalid && (
        <p id={`${question.id}-error`} className="question__error" role="alert">
          This one is required.
        </p>
      )}
    </fieldset>
  );
}

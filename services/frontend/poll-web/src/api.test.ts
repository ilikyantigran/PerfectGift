import { describe, expect, it, vi } from "vitest";
import {
  fetchPoll,
  submitResponse,
  PollUnavailableError,
  RateLimitedError,
  PollApiError,
} from "./api";

const BASE = "http://gw.test";

// Build a fake fetch that returns a single canned Response.
function fakeFetch(res: Partial<Response> & { jsonBody?: unknown }) {
  const response = {
    ok: res.ok ?? true,
    status: res.status ?? 200,
    headers: res.headers ?? new Headers(),
    json: async () => res.jsonBody,
    ...res,
  } as unknown as Response;
  return vi.fn(async () => response) as unknown as typeof fetch;
}

describe("fetchPoll", () => {
  it("requests the token route with no Authorization header", async () => {
    const spy = fakeFetch({
      ok: true,
      status: 200,
      jsonBody: { poll_id: "p1", title: "Hi", questions: [] },
    });
    await fetchPoll("TOK EN/1", { baseUrl: BASE, fetchImpl: spy });

    expect(spy).toHaveBeenCalledTimes(1);
    const [url, init] = (spy as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toBe(`${BASE}/v1/polls/token/TOK%20EN%2F1`);
    expect((init as RequestInit).method).toBe("GET");
    const headers = (init as RequestInit).headers as Record<string, string>;
    expect(Object.keys(headers).map((k) => k.toLowerCase())).not.toContain("authorization");
  });

  it("normalizes questions and enum types from the wire shape", async () => {
    const spy = fakeFetch({
      ok: true,
      status: 200,
      jsonBody: {
        poll_id: "p1",
        title: "Your gift",
        questions: [
          { id: "q1", prompt: "Free text?", type: "QUESTION_TYPE_TEXT", required: true },
          {
            id: "q2",
            prompt: "Pick one",
            type: "QUESTION_TYPE_SINGLE_CHOICE",
            options: [{ id: "a", label: "A" }, { id: "b", label: "B" }],
          },
          { id: "q3", prompt: "Pick many", type: 3, options: [] },
        ],
      },
    });

    const poll = await fetchPoll("t", { baseUrl: BASE, fetchImpl: spy });
    expect(poll.pollId).toBe("p1");
    expect(poll.title).toBe("Your gift");
    expect(poll.questions).toHaveLength(3);
    expect(poll.questions[0]).toMatchObject({ type: "text", required: true });
    expect(poll.questions[1].type).toBe("single_choice");
    expect(poll.questions[1].options).toEqual([
      { id: "a", label: "A" },
      { id: "b", label: "B" },
    ]);
    expect(poll.questions[2].type).toBe("multi_choice");
    expect(poll.questions[2].required).toBe(false);
  });

  it("throws PollUnavailableError on 404 (expired/invalid/revoked)", async () => {
    const spy = fakeFetch({ ok: false, status: 404 });
    await expect(fetchPoll("t", { baseUrl: BASE, fetchImpl: spy })).rejects.toBeInstanceOf(
      PollUnavailableError,
    );
  });

  it("throws RateLimitedError on 429 and parses Retry-After", async () => {
    const spy = fakeFetch({
      ok: false,
      status: 429,
      headers: new Headers({ "Retry-After": "30" }),
    });
    const err = await fetchPoll("t", { baseUrl: BASE, fetchImpl: spy }).catch((e) => e);
    expect(err).toBeInstanceOf(RateLimitedError);
    expect((err as RateLimitedError).retryAfterSeconds).toBe(30);
  });

  it("throws PollApiError on 5xx", async () => {
    const spy = fakeFetch({ ok: false, status: 502 });
    const err = await fetchPoll("t", { baseUrl: BASE, fetchImpl: spy }).catch((e) => e);
    expect(err).toBeInstanceOf(PollApiError);
    expect((err as PollApiError).status).toBe(502);
  });

  it("throws PollApiError when the network call itself fails", async () => {
    const spy = vi.fn(async () => {
      throw new Error("offline");
    }) as unknown as typeof fetch;
    await expect(fetchPoll("t", { baseUrl: BASE, fetchImpl: spy })).rejects.toBeInstanceOf(
      PollApiError,
    );
  });
});

describe("submitResponse", () => {
  it("POSTs answers wrapped in { answers } as JSON", async () => {
    const spy = fakeFetch({ ok: true, status: 200, jsonBody: { ok: true } });
    const answers = [
      { question_id: "q1", text: "blue" },
      { question_id: "q2", choice_ids: ["a", "b"] },
    ];
    await submitResponse("tok", answers, { baseUrl: BASE, fetchImpl: spy });

    const [url, init] = (spy as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toBe(`${BASE}/v1/polls/token/tok/responses`);
    expect((init as RequestInit).method).toBe("POST");
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({ answers });
    const headers = (init as RequestInit).headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");
  });

  it("resolves on a 2xx with an empty body", async () => {
    const spy = fakeFetch({
      ok: true,
      status: 200,
      json: async () => {
        throw new Error("no body");
      },
    });
    await expect(
      submitResponse("tok", [], { baseUrl: BASE, fetchImpl: spy }),
    ).resolves.toBeUndefined();
  });

  it("throws PollApiError when the server returns ok:false", async () => {
    const spy = fakeFetch({ ok: true, status: 200, jsonBody: { ok: false } });
    await expect(
      submitResponse("tok", [], { baseUrl: BASE, fetchImpl: spy }),
    ).rejects.toBeInstanceOf(PollApiError);
  });

  it("maps 404 and 429 the same way as fetchPoll", async () => {
    const notFound = fakeFetch({ ok: false, status: 404 });
    await expect(
      submitResponse("tok", [], { baseUrl: BASE, fetchImpl: notFound }),
    ).rejects.toBeInstanceOf(PollUnavailableError);

    const limited = fakeFetch({ ok: false, status: 429, headers: new Headers() });
    await expect(
      submitResponse("tok", [], { baseUrl: BASE, fetchImpl: limited }),
    ).rejects.toBeInstanceOf(RateLimitedError);
  });
});

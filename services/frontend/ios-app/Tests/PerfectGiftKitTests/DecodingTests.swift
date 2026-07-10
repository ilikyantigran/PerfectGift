import Testing
import Foundation
@testable import PerfectGiftKit

/// Error-envelope decoding, enum leniency, and snake_case model decoding.
@Suite struct DecodingTests {

    let decoder = JSONCoding.decoder

    @Test func errorEnvelopeDecodes() throws {
        let json = """
        { "error": { "code": "not_found", "message": "No such generation", "details": { "id": "req_9" } } }
        """.data(using: .utf8)!
        let env = try decoder.decode(ErrorEnvelope.self, from: json)
        #expect(env.error.code == "not_found")
        #expect(env.error.message == "No such generation")
        #expect(env.error.details?["id"] == "req_9")
    }

    @Test func errorEnvelopeWithoutDetails() throws {
        let json = #"{ "error": { "code": "rate_limited", "message": "Slow down" } }"#.data(using: .utf8)!
        let env = try decoder.decode(ErrorEnvelope.self, from: json)
        #expect(env.error.code == "rate_limited")
        #expect(env.error.details == nil)
    }

    @Test func makeErrorMapsEnvelopeToServerError() throws {
        let json = #"{ "error": { "code": "forbidden", "message": "Nope" } }"#.data(using: .utf8)!
        let env = try decoder.decode(ErrorEnvelope.self, from: json)
        let apiError = APIError.server(status: 403, code: env.error.code, message: env.error.message, details: env.error.details)
        #expect(apiError.code == "forbidden")
        #expect(apiError.userMessage == "Nope")
    }

    @Test func generationStatusFullEnumNames() {
        #expect(GenerationStatus.parse("GENERATION_STATUS_READY") == .ready)
        #expect(GenerationStatus.parse("GENERATION_STATUS_RUNNING") == .running)
        #expect(GenerationStatus.parse("running") == .running)          // lenient short alias
        #expect(GenerationStatus.parse("totally_new_value") == .unknown)
        #expect(GenerationStatus.ready.isTerminal)
        #expect(!GenerationStatus.queued.isTerminal)
    }

    @Test func providerEncodesToFullProtoName() throws {
        let data = try JSONCoding.encoder.encode(SignInRequest.apple(idToken: "tok"))
        let str = String(data: data, encoding: .utf8)!
        #expect(str.contains("\"PROVIDER_APPLE\""))   // full proto name, not "apple"
        #expect(str.contains("\"id_token\""))          // snake_case field names
    }

    @Test func tierEncodesToFullProtoName() throws {
        let req = RequestGenerationRequest(holidayId: "h1", tier: .opus)
        let str = String(data: try JSONCoding.encoder.encode(req), encoding: .utf8)!
        #expect(str.contains("\"MODEL_TIER_OPUS\""))
        #expect(str.contains("\"holiday_id\""))
    }

    @Test func generationResultDecodesSnakeCaseAndRanks() throws {
        let json = """
        {
          "request_id": "req_1",
          "status": "GENERATION_STATUS_READY",
          "progress": 100,
          "ideas": [
            { "id": "i2", "title": "Spa day", "why_it_fits": "Relaxing", "rough_cost": "$120", "rank": 2 },
            { "id": "i1", "title": "Hot air balloon", "why_it_fits": "Adventurous", "rank": 1 }
          ]
        }
        """.data(using: .utf8)!
        let result = try decoder.decode(GenerationResult.self, from: json)
        #expect(result.status == .ready)
        #expect(result.progress == 100)
        #expect(result.rankedIdeas.first?.id == "i1")            // rank 1 sorts first
        #expect(result.rankedIdeas.first?.whyItFits == "Adventurous")
    }

    @Test func tokenPairDecodesUserAndExpiry() throws {
        let json = """
        { "access_token": "acc", "refresh_token": "ref", "expires_in": 3600,
          "user": { "id": "u1", "email": "a@b.co", "display_name": "Ada" } }
        """.data(using: .utf8)!
        let pair = try decoder.decode(TokenPair.self, from: json)
        #expect(pair.accessToken == "acc")
        #expect(pair.expiresIn == 3600)
        #expect(pair.user?.displayName == "Ada")
    }

    @Test func questionKindLenientDecoding() throws {
        let json = #"{ "id": "q1", "text": "Vibe?", "kind": "QUESTION_TYPE_MULTI_CHOICE", "options": ["A","B"] }"#.data(using: .utf8)!
        let q = try decoder.decode(Question.self, from: json)
        #expect(q.kind == .multiChoice)
        #expect(q.kind.isChoice)
    }
}

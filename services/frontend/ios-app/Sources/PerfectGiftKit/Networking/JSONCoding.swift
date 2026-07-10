import Foundation

/// Shared JSON coders. The gateway speaks snake_case; our models declare explicit
/// `CodingKeys`, so we do NOT use `.convertToSnakeCase` (that would double-convert).
public enum JSONCoding {
    public static let encoder: JSONEncoder = {
        let e = JSONEncoder()
        e.outputFormatting = [.withoutEscapingSlashes]
        return e
    }()

    public static let decoder: JSONDecoder = {
        let d = JSONDecoder()
        return d
    }()
}

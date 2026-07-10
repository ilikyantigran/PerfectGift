import Foundation

/// A holiday reference entry (`GET /v1/holidays`).
public struct Holiday: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let name: String
    public let dateRule: String?
    public let region: String?
    public let tags: [String]?
    public let active: Bool?

    public init(id: String, name: String, dateRule: String? = nil, region: String? = nil, tags: [String]? = nil, active: Bool? = nil) {
        self.id = id
        self.name = name
        self.dateRule = dateRule
        self.region = region
        self.tags = tags
        self.active = active
    }

    private enum CodingKeys: String, CodingKey {
        case id, name, region, tags, active
        case dateRule = "date_rule"
    }
}

/// Wrapper for `GET /v1/holidays`.
public struct HolidaysResponse: Codable, Sendable, Equatable {
    public let holidays: [Holiday]
    public init(holidays: [Holiday]) { self.holidays = holidays }
}

/// A gift/date category (`GET /v1/categories`).
public struct Category: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let name: String
    public let kind: String?
    public let parentId: String?

    public init(id: String, name: String, kind: String? = nil, parentId: String? = nil) {
        self.id = id
        self.name = name
        self.kind = kind
        self.parentId = parentId
    }

    private enum CodingKeys: String, CodingKey {
        case id, name, kind
        case parentId = "parent_id"
    }
}

/// A budget band (`GET /v1/categories`).
public struct BudgetBand: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let label: String
    public let min: Int64?
    public let max: Int64?
    public let currency: String?

    public init(id: String, label: String, min: Int64? = nil, max: Int64? = nil, currency: String? = nil) {
        self.id = id
        self.label = label
        self.min = min
        self.max = max
        self.currency = currency
    }
}

/// Wrapper for `GET /v1/categories`.
public struct CategoriesResponse: Codable, Sendable, Equatable {
    public let categories: [Category]
    public let budgetBands: [BudgetBand]

    public init(categories: [Category] = [], budgetBands: [BudgetBand] = []) {
        self.categories = categories
        self.budgetBands = budgetBands
    }

    private enum CodingKeys: String, CodingKey {
        case categories
        case budgetBands = "budget_bands"
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        categories = try c.decodeIfPresent([Category].self, forKey: .categories) ?? []
        budgetBands = try c.decodeIfPresent([BudgetBand].self, forKey: .budgetBands) ?? []
    }
}

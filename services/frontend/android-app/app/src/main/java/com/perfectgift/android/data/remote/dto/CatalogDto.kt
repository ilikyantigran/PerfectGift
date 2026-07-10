package com.perfectgift.android.data.remote.dto

data class HolidayDto(
    val id: String? = null,
    val name: String? = null,
    val dateRule: String? = null,
    val region: String? = null,
    val tags: List<String>? = null,
    val active: Boolean = false,
)

data class HolidaysResponse(
    val holidays: List<HolidayDto> = emptyList(),
)

data class CategoryDto(
    val id: String? = null,
    val name: String? = null,
    val kind: String? = null,
    val parentId: String? = null,
)

data class BudgetBandDto(
    val id: String? = null,
    val label: String? = null,
    val min: Long = 0,
    val max: Long = 0,
    val currency: String? = null,
)

/** GET /v1/categories — categories + budget bands. */
data class CategoriesResponse(
    val categories: List<CategoryDto> = emptyList(),
    val budgetBands: List<BudgetBandDto> = emptyList(),
)

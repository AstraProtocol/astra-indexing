package pagination

import "math"

// Pagination stores pagination request data
type Pagination struct {
	t            string
	offsetParams PaginationOffsetParams
}

func NewOffsetPagination(page int64, limit int64) *Pagination {
	return &Pagination{
		t: PAGINATION_OFFSET,

		offsetParams: PaginationOffsetParams{
			Page:  page,
			Limit: limit,
		},
	}
}

func (pagination *Pagination) Type() string {
	return pagination.t
}

func (pagination *Pagination) OffsetParams() *PaginationOffsetParams {
	if pagination.Type() != PAGINATION_OFFSET {
		return nil
	}
	return &pagination.offsetParams
}

// TODO: Cursor pagination
// func (pagination *Pagination) CursorParams() *PaginationCursorParams {
// 	return &pagination.cursorParams
// }

func (pagination *Pagination) OffsetResult(totalRecord int64) *Result {
	if pagination.Type() != PAGINATION_OFFSET {
		return nil
	}
	return NewOffsetPaginationResult(
		totalRecord,
		pagination.offsetParams.Page,
		pagination.offsetParams.Limit,
	)
}

type PaginationOffsetParams struct {
	Page  int64
	Limit int64
}

func (params *PaginationOffsetParams) Offset() int64 {
	return params.Limit * (params.Page - 1)
}

type Result struct {
	T string `json:"t"`

	Por OffsetResult `json:"pagination_offset_result"`
}

func NewOffsetPaginationResult(totalRecord int64, currentPage int64, limit int64) *Result {
	return &Result{
		T: PAGINATION_OFFSET,

		Por: OffsetResult{
			TotalRecord: totalRecord,
			CurrentPage: currentPage,
			Limit:       limit,
		},
	}
}

func (result *Result) Type() string {
	return result.T
}

func (result *Result) OffsetResult() *OffsetResult {
	if result.Type() != PAGINATION_OFFSET {
		return nil
	}
	return &result.Por
}

type OffsetResult struct {
	TotalRecord int64 `json:"total_record"`
	CurrentPage int64 `json:"current_page"`
	Limit       int64 `json:"limit"`
}

func (result *OffsetResult) TotalPage() int64 {
	return int64(math.Ceil(float64(result.TotalRecord) / float64(result.Limit)))
}

const (
	PAGINATION_OFFSET string = "offset"
	// PAGINATION_CURSOR
)

const (
	MAX_ELEMENTS int64 = 50000
)

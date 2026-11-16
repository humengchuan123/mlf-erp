package masterdatahttp

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
)

type partnerRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Contact  string `json:"contact"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Address  string `json:"address"`
	TaxNo    string `json:"tax_no"`
}

type materialRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Spec      string `json:"spec"`
	Category  string `json:"category"`
	UnitID    string `json:"unit_id"`
	TaxRateID string `json:"tax_rate_id"`
	Barcode   string `json:"barcode"`
}

type warehouseRequest struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

type unitRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Precision int    `json:"precision"`
}

type currencyRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	DecimalPlaces int    `json:"decimal_places"`
}

type taxRateRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Rate string `json:"rate"`
	Type string `json:"type"`
}

func NewPartnerHandler(service *app.PartnerService, audit *systemapp.AuditService, log *slog.Logger) *CrudHandler[domain.Partner, app.PartnerInput] {
	return NewCrudHandler[domain.Partner, app.PartnerInput](
		service, "/masterdata/partners", "masterdata.partner", "partner", decodePartner,
		func(item domain.Partner) any { return item }, audit, log,
	)
}

func NewMaterialHandler(service *app.MaterialService, audit *systemapp.AuditService, log *slog.Logger) *CrudHandler[domain.Material, app.MaterialInput] {
	return NewCrudHandler[domain.Material, app.MaterialInput](
		service, "/masterdata/materials", "masterdata.material", "material", decodeMaterial,
		func(item domain.Material) any { return item }, audit, log,
	)
}

func NewWarehouseHandler(service *app.WarehouseService, audit *systemapp.AuditService, log *slog.Logger) *CrudHandler[domain.Warehouse, app.WarehouseInput] {
	return NewCrudHandler[domain.Warehouse, app.WarehouseInput](
		service, "/masterdata/warehouses", "masterdata.warehouse", "warehouse", decodeWarehouse,
		func(item domain.Warehouse) any { return item }, audit, log,
	)
}

func NewUnitHandler(service *app.UnitService, audit *systemapp.AuditService, log *slog.Logger) *CrudHandler[domain.Unit, app.UnitInput] {
	return NewCrudHandler[domain.Unit, app.UnitInput](
		service, "/masterdata/units", "masterdata.unit", "unit", decodeUnit,
		func(item domain.Unit) any { return item }, audit, log,
	)
}

func NewCurrencyHandler(service *app.CurrencyService, audit *systemapp.AuditService, log *slog.Logger) *CrudHandler[domain.Currency, app.CurrencyInput] {
	return NewCrudHandler[domain.Currency, app.CurrencyInput](
		service, "/masterdata/currencies", "masterdata.currency", "currency", decodeCurrency,
		func(item domain.Currency) any { return item }, audit, log,
	)
}

func NewTaxRateHandler(service *app.TaxRateService, audit *systemapp.AuditService, log *slog.Logger) *CrudHandler[domain.TaxRate, app.TaxRateInput] {
	return NewCrudHandler[domain.TaxRate, app.TaxRateInput](
		service, "/masterdata/tax-rates", "masterdata.tax", "tax_rate", decodeTaxRate,
		func(item domain.TaxRate) any { return item }, audit, log,
	)
}

func decodePartner(c *gin.Context) (app.PartnerInput, error) {
	var request partnerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.PartnerInput{}, err
	}
	return app.PartnerInput{
		Code: request.Code, Name: request.Name, Type: request.Type, Category: request.Category,
		Contact: request.Contact, Phone: request.Phone, Email: request.Email,
		Address: request.Address, TaxNo: request.TaxNo,
	}, nil
}

func decodeMaterial(c *gin.Context) (app.MaterialInput, error) {
	var request materialRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.MaterialInput{}, err
	}
	return app.MaterialInput{
		Code: request.Code, Name: request.Name, Spec: request.Spec, Category: request.Category,
		UnitID: request.UnitID, TaxRateID: request.TaxRateID, Barcode: request.Barcode,
	}, nil
}

func decodeWarehouse(c *gin.Context) (app.WarehouseInput, error) {
	var request warehouseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.WarehouseInput{}, err
	}
	return app.WarehouseInput{
		Code: request.Code, Name: request.Name, Type: request.Type, Address: request.Address,
	}, nil
}

func decodeUnit(c *gin.Context) (app.UnitInput, error) {
	var request unitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.UnitInput{}, err
	}
	return app.UnitInput{Code: request.Code, Name: request.Name, Precision: request.Precision}, nil
}

func decodeCurrency(c *gin.Context) (app.CurrencyInput, error) {
	var request currencyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.CurrencyInput{}, err
	}
	return app.CurrencyInput{
		Code: request.Code, Name: request.Name, Symbol: request.Symbol, DecimalPlaces: request.DecimalPlaces,
	}, nil
}

func decodeTaxRate(c *gin.Context) (app.TaxRateInput, error) {
	var request taxRateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.TaxRateInput{}, err
	}
	return app.TaxRateInput{
		Code: request.Code, Name: request.Name, Rate: request.Rate, Type: request.Type,
	}, nil
}

package install

import (
	"cmp"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
)

type SetupRequest struct {
	prefix       string
	sourceRunner string
	fontCode     string
}

func NewSetupRequest(prefix, sourceRunner, fontCode string) SetupRequest {
	return SetupRequest{prefix: prefix, sourceRunner: sourceRunner, fontCode: fontCode}
}

func (request SetupRequest) ChoosesSourceRunner() bool {
	return request.sourceRunner != ""
}

func (request SetupRequest) SourceRunner() string {
	return request.sourceRunner
}

func (request SetupRequest) AppliedTo(base Installation) (Installation, error) {
	fontVariant, err := request.fontVariantOr(base.fontVariant)
	if err != nil {
		return base, err
	}
	return New(cmp.Or(request.prefix, base.prefix), cmp.Or(request.sourceRunner, base.sourceRunner), fontVariant, base.executable, base.launcher)
}

func (request SetupRequest) fontVariantOr(fallback fonts.Variant) (fonts.Variant, error) {
	if request.fontCode == "" {
		return fallback, nil
	}
	return fonts.ParseVariant(request.fontCode)
}

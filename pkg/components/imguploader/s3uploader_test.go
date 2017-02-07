package imguploader

import (
	"testing"

	"github.com/grafana/grafana/pkg/setting"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUploadToS3(t *testing.T) {
	SkipConvey("[Integration test] for external_image_store.webdav", t, func() {
		setting.NewConfigContext(&setting.CommandLineArgs{
			HomePath: "../../../",
		})

		s3Uploader, _ := NewImageUploader()

		path, err := s3Uploader.Upload("../../../public/img/logo_transparent_400x.png")

		So(err, ShouldBeNil)
		So(path, ShouldNotEqual, "")
	})
}

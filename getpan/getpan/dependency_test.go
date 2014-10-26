package getpan

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func doTest(t *testing.T, Name, Modifier, Version string) {
	Convey("DependencyFromString", t, func() {
		dv, err := DependencyFromString(Name, Modifier+" "+Version)
		So(dv.Name, ShouldEqual, Name)
		So(dv.Version, ShouldEqual, Version)
		So(dv.Modifier, ShouldEqual, Modifier)
		So(err, ShouldBeNil)
	})
}

func TestVersionFromString(t *testing.T) {
	doTest(t, "Mojolicious", ">=", "0.05")
	doTest(t, "Mojolicious", "==", "0.07")
	doTest(t, "Mojolicious", "<=", "0.09")
	doTest(t, "Mojolicious", ">", "0.14")
	doTest(t, "Mojolicious", "<", "0.20")
}

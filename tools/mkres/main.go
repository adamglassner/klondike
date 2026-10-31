// mkres writes a Windows resource .syso (icon, version info, manifest) so the
// launcher .exe shows the right icon and version in Explorer.
//
//	mkres -version 1.0.0 -icon assets/icon-1024.png -out launcher/rsrc_windows_amd64.syso
package main

import (
	"flag"
	"image"
	_ "image/png"
	"log"
	"os"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func main() {
	ver := flag.String("version", "0.0.0", "semantic version")
	iconPath := flag.String("icon", "", "PNG icon")
	out := flag.String("out", "rsrc_windows_amd64.syso", "output .syso")
	name := flag.String("name", "Ironite Launcher", "product name")
	exe := flag.String("exe", "IroniteLauncher.exe", "original file name")
	internal := flag.String("internal", "IroniteLauncher", "internal name")
	identity := flag.String("identity", "Ironite.Launcher", "manifest identity")
	desc := flag.String("desc", "", "file description (defaults to the product name)")
	flag.Parse()

	rs := winres.ResourceSet{}

	f, err := os.Open(*iconPath)
	if err != nil {
		log.Fatal(err)
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	icon, err := winres.NewIconFromResizedImage(img, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := rs.SetIcon(winres.Name("APPICON"), icon); err != nil {
		log.Fatal(err)
	}

	vi := version.Info{}
	vi.SetFileVersion(*ver)
	vi.SetProductVersion(*ver)
	for k, v := range map[string]string{
		version.CompanyName:      "Ironite",
		version.FileDescription:  firstNonEmpty(*desc, *name),
		version.ProductName:      *name,
		version.FileVersion:      *ver,
		version.ProductVersion:   *ver,
		version.LegalCopyright:   "Copyright (c) 2026 Ironite. All rights reserved.",
		version.OriginalFilename: *exe,
		version.InternalName:     *internal,
	} {
		_ = vi.Set(version.LangDefault, k, v)
	}
	rs.SetVersionInfo(vi)

	rs.SetManifest(winres.AppManifest{
		Identity:            winres.AssemblyIdentity{Name: *identity},
		Description:         *name,
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
		ExecutionLevel:      winres.AsInvoker,
	})

	o, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer o.Close()
	if err := rs.WriteObject(o, winres.ArchAMD64); err != nil {
		log.Fatal(err)
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

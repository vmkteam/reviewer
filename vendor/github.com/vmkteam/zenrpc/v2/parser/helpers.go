package parser

import (
	"errors"
	"go/ast"
	"path"

	"golang.org/x/tools/go/packages"
)

type PackageFiles struct {
	PackagePath string
	PackageName string

	AstFiles []*ast.File
}

func GetDependenciesAstFiles(filename string) ([]PackageFiles, error) {
	pkgs, err := loadPackageWithSyntax(path.Dir(filename))
	if err != nil {
		return nil, err
	}
	pfs := []PackageFiles{}
	done := map[string]bool{}
	for _, pkg := range pkgs {
		if _, ok := done[pkg.PkgPath]; ok {
			continue
		}

		pfs = append(pfs, PackageFiles{
			PackagePath: pkg.PkgPath,
			PackageName: pkg.Name,
			AstFiles:    pkg.Syntax,
		})

		done[pkg.PkgPath] = true

		for _, childPack := range pkg.Imports {
			if _, ok := done[childPack.PkgPath]; ok {
				continue
			}

			pfs = append(pfs, PackageFiles{
				PackagePath: childPack.PkgPath,
				PackageName: childPack.Name,
				AstFiles:    childPack.Syntax,
			})

			done[childPack.PkgPath] = true
		}
	}
	return pfs, nil
}

func EntryPointPackageName(filename string) (string, string, error) {
	pkgs, err := loadPackage(path.Dir(filename))
	if err != nil {
		return "", "", err
	}
	for _, pack := range pkgs {
		return pack.Name, pack.PkgPath, nil
	}
	return "", "", errors.New("package not found for entry point")
}

func loadPackage(path string) ([]*packages.Package, error) {
	return packages.Load(&packages.Config{
		Mode: packages.NeedImports | packages.NeedFiles | packages.NeedDeps | packages.NeedName,
	}, path)
}

func loadPackageWithSyntax(path string) ([]*packages.Package, error) {
	return packages.Load(&packages.Config{
		Mode: packages.NeedImports |
			packages.NeedFiles |
			packages.NeedDeps |
			packages.NeedName |
			packages.NeedSyntax,
	}, path)
}

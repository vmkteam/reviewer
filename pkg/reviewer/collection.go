package reviewer

//go:generate colgen -imports=reviewsrv/pkg/db
//colgen:Review,ReviewFile,Issue,Project
//colgen:Project:MapP(db)
//colgen:Issue:MapP(db),Group(ReviewFileID)
//colgen:ReviewFile:MapP(db),Group(ReviewID)
//colgen:Review:MapP(db)

// MapP converts slice of type T to slice of type M with given converter with pointers.
func MapP[T, M any](a []T, f func(*T) *M) []M {
	n := make([]M, len(a))
	for i := range a {
		n[i] = *f(&a[i])
	}
	return n
}

// Map converts slice of type T to slice of type M with given converter.
func Map[T, M any](a []T, f func(T) M) []M {
	n := make([]M, len(a))
	for i := range a {
		n[i] = f(a[i])
	}
	return n
}

// Ptr is a generic to create pointer from value
func Ptr[T any](v T) *T {
	return &v
}

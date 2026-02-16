package rpc

//go:generate colgen -funcpkg=reviewer -imports=reviewsrv/pkg/reviewer
//colgen:Project:mapp(reviewer)
//colgen:Issue:mapp(reviewer)
//colgen:ReviewFile:mapp(reviewer)
//colgen:Review:mapp(reviewer)
//colgen:ReviewSummary:mapp(reviewer.Review)

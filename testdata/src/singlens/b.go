//declscope:namespace shared

package singlens

func use() int { return helper() + count }

var _ = use

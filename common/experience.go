package common

import "strings"

const ExperienceOrderPrefix = "EXO-"

func IsExperienceOrder(orderNo string) bool { return strings.HasPrefix(orderNo, ExperienceOrderPrefix) }

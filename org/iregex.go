package org

import "regexp"

type IRegEx map[string]string

func MatchIRegExStr(regEx, in string) IRegEx {

	var compRegEx = regexp.MustCompile(regEx)
	match := compRegEx.FindStringSubmatch(in)

	if match == nil {
		return nil
	}

	var paramsMap = make(IRegEx)
	for i, name := range compRegEx.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	return paramsMap
}

func MatchIRegEx(compRegEx *regexp.Regexp, in string) IRegEx {

	match := compRegEx.FindStringSubmatch(in)

	if match == nil {
		return nil
	}

	var paramsMap = make(IRegEx)
	for i, name := range compRegEx.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	return paramsMap
}

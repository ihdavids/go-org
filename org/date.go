package org

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"
)

/*
   """
   Generate timetamp regex for active/inactive/nobrace brace type

   :type brtype: {'active', 'inactive', 'nobrace'}
   :arg  brtype:
       It specifies a type of brace.
       active: <>-type; inactive: []-type; nobrace: no braces.

   :type prefix: str or None
   :arg  prefix:
       It will be appended to the head of keys of the "groupdict".
       For example, if prefix is ``'active_'`` the groupdict has
       keys such as ``'active_year'``, ``'active_month'``, and so on.
       If it is None it will be set to ``brtype`` + ``'_'``.

   :type nocookie: bool
   :arg  nocookie:
       Cookie part (e.g., ``'-3d'`` or ``'+6m'``) is not included if
       it is ``True``.  Default value is ``False``.

   >>> timestamp_re = re.compile(
   ...     gene_timestamp_regex('active', prefix=''),
   ...     re.VERBOSE)
   >>> timestamp_re.match('no match')  # returns None
   >>> m = timestamp_re.match('<2010-06-21 Mon>')
   >>> m.group()
   '<2010-06-21 Mon>'
   >>> '{year}-{month}-{day}'.format(**m.groupdict())
   '2010-06-21'
   >>> m = timestamp_re.match('<2005-10-01 Sat 12:30 +7m -3d>')
   >>> from collections import OrderedDict
   >>> sorted(m.groupdict().items())
   ... # doctest: +NORMALIZE_WHITESPACE
   [('day', '01'),
    ('end_hour', None), ('end_min', None),
    ('hour', '12'), ('min', '30'),
    ('month', '10'),
    ('repeatdwmy', 'm'), ('repeatnum', '7'), ('repeatpre', '+'),
    ('warndwmy', 'd'), ('warnnum', '3'), ('warnpre', '-'), ('year', '2005')]

   When ``brtype = 'nobrace'``, cookie part cannot be retrieved.

   >>> timestamp_re = re.compile(
   ...     gene_timestamp_regex('nobrace', prefix=''),
   ...     re.VERBOSE)
   >>> timestamp_re.match('no match')  # returns None
   >>> m = timestamp_re.match('2010-06-21 Mon')
   >>> m.group()
   '2010-06-21'
   >>> '{year}-{month}-{day}'.format(**m.groupdict())
   '2010-06-21'
   >>> m = timestamp_re.match('2005-10-01 Sat 12:30 +7m -3d')
   >>> sorted(m.groupdict().items())
   ... # doctest: +NORMALIZE_WHITESPACE
   [('day', '01'),
    ('end_hour', None), ('end_min', None),
    ('hour', '12'), ('min', '30'),
    ('month', '10'), ('year', '2005')]
   """
*/
func GenTimestampRegex(brtype string, prefix string, nocookie bool) string {
	bo, bc := "", ""
	if brtype == "active" {
		bo, bc = `\<`, `\>`
	} else if brtype == "inactive" {
		bo, bc = `\[`, `\]`
	}

	var ignore = ""
	if brtype == "nobrace" {
		ignore = `[\s\w]`
	} else {
		ignore = fmt.Sprintf(`[^%s]`, bc)
	}

	/*
		if prefix == "" {
			prefix = fmt.Sprintf(`%s_`, brtype)
		}
	*/

	var regex_date_time = `(?P<{{.prefix}}year>\d{4}) *\- *(?P<{{.prefix}}month>\d{2}) *\- *(?P<{{.prefix}}day>\d{2}) *(({{.ignore}}+?)(?P<{{.prefix}}hour>\d{2}) *: *(?P<{{.prefix}}min>\d{2})( *\-\-? *(?P<{{.prefix}}end_hour>\d{2}) : *(?P<{{.prefix}}end_min>\d{2}))?)?`

	var regex_cookie = `(({ignore}+?)(?P<{{.prefix}}repeatpre> *[\.\+]{1,2})(?P<{{.prefix}}repeatnum> *\d+)(?P<{{.prefix}}repeatdwmy> *[dwmy]))?(({{.ignore}}+?)(?P<{{.prefix}}warnpre> *\-)(?P<{{.prefix}}warnnum> *\d+)(?P<{{.prefix}}warndwmy> *[dwmy]))?`

	// http://www.pythonregex.com/
	if !(nocookie || brtype != "nobrace") {
		regex_cookie = ""
	}
	var mm = map[string]interface{}{
		"prefix": prefix,
		"ignore": ignore,
	}
	restr, _ := AString(strings.Join([]string{bo, regex_date_time, regex_cookie, "({{.ignore}}*?)", bc}, "")).Format(mm)
	return restr
}

type DateParser struct {
	Re     regexp.Regexp
	Active bool
}

func CompileSDCRe(sdctype string) *DateParser {
	var brtype = "active"
	if sdctype == "CLOSED" {
		brtype = "inactive"
	}
	var mm = map[string]interface{}{
		"sdctype": sdctype,
		"timere":  GenTimestampRegex(brtype, "", true),
	}
	var tmpl string = `^[^#]*{{.sdctype}}:\s+{{.timere}}`
	tmp, err := AString(tmpl).Format(mm)
	if err == nil {
		var dp *DateParser = new(DateParser)
		dp.Re = *regexp.MustCompile(tmp)
		dp.Active = brtype == "active"
		return dp
	}
	return nil
}

var OrgDateScheduled = CompileSDCRe("SCHEDULED")
var OrgDateDeadline = CompileSDCRe("DEADLINE")
var OrgDateClosed = CompileSDCRe("CLOSED")

//    def _datetuple_from_groupdict(cls, dct, prefix=''):
//        return cls._daterange_from_groupdict(dct, prefix=prefix)[0]

func DateRangeFromGroupDict(dct IRegEx, prefix string) ([]string, []string, bool) {
	start_keys := []string{"year", "month", "day", "hour", "min"}
	end_keys := []string{"year", "month", "day", "end_hour", "end_min"}
	start_range := []string{}
	end_range := []string{}
	for _, x := range start_keys {
		key := prefix + x
		if v, ok := dct[key]; ok {
			start_range = append(start_range, v)
		}
	}
	for _, x := range end_keys {
		key := prefix + x
		if v, ok := dct[key]; ok {
			start_range = append(start_range, v)
		}
	}
	if len(end_range) < len(end_keys) {
		end_range = nil
	}
	return start_range, end_range, len(start_range) < len(start_keys)
}

func orgDateFromTuple(tpl []string) time.Time {
	// year, month, day, hour, min
	l := len(tpl)
	if l < 3 {
		return time.Time{}
	}
	var year, month, day, hour, min int
	var err error
	if year, err = strconv.Atoi(tpl[0]); err != nil {
		return time.Time{}
	}
	if month, err = strconv.Atoi(tpl[1]); err != nil {
		return time.Time{}
	}
	if day, err = strconv.Atoi(tpl[2]); err != nil {
		return time.Time{}
	}
	if l == 5 {
		if hour, err = strconv.Atoi(tpl[3]); err != nil {
			return time.Time{}
		}
		if min, err = strconv.Atoi(tpl[4]); err != nil {
			return time.Time{}
		}
		return time.Date(year, time.Month(month), day, hour, min, 0, 0, time.Local)
	} else {
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	}
}

type OrgDate struct {
	Start      time.Time
	End        time.Time
	Active     bool
	HaveTime   bool
	RepeatRule *rrule.RRule
	RepeatPre  string
	RepeatDWMY string
	WarnRule   *rrule.RRule
	WarnPre    string
	WarnDWMY   string
	/*
	   self._start = self._to_date(start)
	   self._end = self._to_date(end)
	   self._active = self._active_default if active is None else active
	   self.repeat_rule = repeat_rule
	   self.warn_rule = warn_rule
	*/
}

func NewOrgDateFromTuple(start, end []string, active bool) *OrgDate {
	d := new(OrgDate)
	d.Start = orgDateFromTuple(start)
	d.End = orgDateFromTuple(end)
	d.Active = active
	return d
}

func (self *DateParser) Parse(line string) *OrgDate {
	match := MatchIRegEx(&self.Re, line)
	if match != nil {
		start, end, haveTime := DateRangeFromGroupDict(match, "")
		orgDate := NewOrgDateFromTuple(start, end, self.Active)
		orgDate.HaveTime = haveTime
		repeatpre, rpok := match["repeatpre"]
		repeatnum, rnok := match["repeatnum"]
		repeatdwmy, rdok := match["repeatdwmy"]
		if rdok && rpok && rnok {
			freq := rrule.DAILY
			if repeatdwmy == "y" {
				freq = rrule.YEARLY
			}
			if repeatdwmy == "m" {
				freq = rrule.MONTHLY
			}
			if repeatdwmy == "w" {
				freq = rrule.WEEKLY
			}
			var rnum int = 0
			var err error
			if rnum, err = strconv.Atoi(repeatnum); err != nil || rnum <= 0 {
				rnum = 1
			}
			rr, _ := rrule.NewRRule(rrule.ROption{
				Freq:     freq,
				Interval: rnum,
				/*
					Count:      3,
					Bymonth:    []int{11},
					Byweekday:  []rrule.Weekday{rrule.TU},
					Bymonthday: []int{2, 3, 4, 5, 6, 7, 8},
				*/
				Dtstart: orgDate.Start})
			orgDate.RepeatRule = rr
			// This determines what to do when you mark the task as done.
			// + just bump to the next FIXED interval (even if thats in the past)
			// ++ bump to the next FIXED interval, in the future. (IE next sunday) even if you missed some.
			// .+ bump but change the start date to today.
			orgDate.RepeatPre = repeatpre
			orgDate.RepeatDWMY = repeatdwmy
		}
		warnpre, wpok := match["warnpre"]
		warnnum, wnok := match["warnnum"]
		warndwmy, wdok := match["warndwmy"]
		if wdok && wpok && wnok {
			freq := rrule.DAILY
			if warndwmy == "y" {
				freq = rrule.YEARLY
			}
			if warndwmy == "m" {
				freq = rrule.MONTHLY
			}
			if warndwmy == "w" {
				freq = rrule.WEEKLY
			}
			var rnum int = 0
			var err error
			if rnum, err = strconv.Atoi(warnnum); err != nil || rnum <= 0 {
				rnum = 1
			}
			rr, _ := rrule.NewRRule(rrule.ROption{
				Freq:     freq,
				Interval: rnum,
				/*
					Count:      3,
					Bymonth:    []int{11},
					Byweekday:  []rrule.Weekday{rrule.TU},
					Bymonthday: []int{2, 3, 4, 5, 6, 7, 8},
				*/
				Dtstart: orgDate.Start})
			orgDate.WarnRule = rr
			// This determines what to do when you mark the task as done.
			// + just bump to the next FIXED interval (even if thats in the past)
			// ++ bump to the next FIXED interval, in the future. (IE next sunday) even if you missed some.
			// .+ bump but change the start date to today.
			orgDate.WarnPre = warnpre
			orgDate.WarnDWMY = warndwmy
		}
		return orgDate
	}
	return nil
}

func (self *OrgDate) After(date time.Time) bool {
	return (self.Start.Before(date) || self.Start.Equal(date))
}

func (self *OrgDate) Before(date time.Time) bool {
	return (self.Start.After(date) || self.Start.Equal(date))
}

func (self *OrgDate) DateTimeInRange(date time.Time) bool {
	return (self.Start.Before(date) || self.Start.Equal(date)) && (self.End.After(date) || self.End.Equal(date))
}

func (self *OrgDate) HasEnd() bool {
	return !self.End.IsZero()
}

func (self *OrgDate) HasTime() bool {
	return self.HaveTime
}

func (self *OrgDate) ToDate() string {
	end := ""
	if !self.End.IsZero() {
		end = " -- " + self.End.Format("2006-01-02 Mon")
	}
	bs, be := "", ""
	if self.Active {
		bs, be = "<", ">"
	}
	return bs + self.Start.Format("2006-01-02 Mon") + end + be
}

func (self *OrgDate) HasOverlap(other *OrgDate) bool {
	if self.HasEnd() {
		return self.DateTimeInRange(other.Start) || self.DateTimeInRange(other.End)
	} else if other.HasEnd() {
		return other.DateTimeInRange(self.Start)
	} else {
		// These could be datetime entries
		// do we care about the hours, probably not!
		// this is containement and we are just a point
		// if these are on the same day we are okay.
		sy, sm, sd := self.Start.Date()
		oy, om, od := other.Start.Date()

		return sy == oy && sm == om && sd == od
	}
}

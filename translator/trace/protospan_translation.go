// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracetranslator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"go.opentelemetry.io/collector/consumer/pdata"
)

// Some of the keys used to represent OTLP constructs as tags or annotations in other formats.
const (
	TagMessage = "message"

	TagSpanKind = "span.kind"

	TagStatusCode          = "status.code"
	TagStatusMsg           = "status.message"
	TagError               = "error"
	TagHTTPStatusMsg       = "http.status_message"
	TagZipkinCensusCode    = "census.status_code"
	TagZipkinCensusMsg     = "census.status_description"
	TagZipkinOpenCensusMsg = "opencensus.status_description"

	TagW3CTraceState     = "w3c.tracestate"
	TagServiceNameSource = "otlp.service.name.source"
)

// Constants used for signifying batch-level attribute values where not supplied by OTLP data but required
// by other protocols.
const (
	ResourceNoServiceName = "OTLPResourceNoServiceName"
)

// OpenTracingSpanKind are possible values for TagSpanKind and match the OpenTracing
// conventions: https://github.com/opentracing/specification/blob/main/semantic_conventions.md
// These values are used for representing span kinds that have no
// equivalents in OpenCensus format. They are stored as values of TagSpanKind
type OpenTracingSpanKind string

const (
	OpenTracingSpanKindUnspecified OpenTracingSpanKind = ""
	OpenTracingSpanKindClient      OpenTracingSpanKind = "client"
	OpenTracingSpanKindServer      OpenTracingSpanKind = "server"
	OpenTracingSpanKindConsumer    OpenTracingSpanKind = "consumer"
	OpenTracingSpanKindProducer    OpenTracingSpanKind = "producer"
	OpenTracingSpanKindInternal    OpenTracingSpanKind = "internal"
)

const (
	SpanLinkDataFormat  = "%s|%s|%s|%s|%d"
	SpanEventDataFormat = "%s|%s|%d"
)

type attrValDescript struct {
	regex    *regexp.Regexp
	attrType pdata.AttributeValueType
}

var attrValDescriptions = getAttrValDescripts()

func getAttrValDescripts() []*attrValDescript {
	descriptions := make([]*attrValDescript, 0, 5)
	descriptions = append(descriptions, constructAttrValDescript("^$", pdata.AttributeValueNULL))
	descriptions = append(descriptions, constructAttrValDescript(`^-?\d+$`, pdata.AttributeValueINT))
	descriptions = append(descriptions, constructAttrValDescript(`^-?\d+\.\d+$`, pdata.AttributeValueDOUBLE))
	descriptions = append(descriptions, constructAttrValDescript(`^(true|false)$`, pdata.AttributeValueBOOL))
	descriptions = append(descriptions, constructAttrValDescript(`^\{"\w+":.+\}$`, pdata.AttributeValueMAP))
	descriptions = append(descriptions, constructAttrValDescript(`^\[.*\]$`, pdata.AttributeValueARRAY))
	return descriptions
}

func constructAttrValDescript(regex string, attrType pdata.AttributeValueType) *attrValDescript {
	regexc := regexp.MustCompile(regex)
	return &attrValDescript{
		regex:    regexc,
		attrType: attrType,
	}
}

// AttributeValueToString converts an OTLP AttributeValue object to its equivalent string representation
func AttributeValueToString(attr pdata.AttributeValue, jsonLike bool) string {
	switch attr.Type() {
	case pdata.AttributeValueNULL:
		if jsonLike {
			return "null"
		}
		return ""
	case pdata.AttributeValueSTRING:
		if jsonLike {
			return fmt.Sprintf("%q", attr.StringVal())
		}
		return attr.StringVal()

	case pdata.AttributeValueBOOL:
		return strconv.FormatBool(attr.BoolVal())

	case pdata.AttributeValueDOUBLE:
		return strconv.FormatFloat(attr.DoubleVal(), 'f', -1, 64)

	case pdata.AttributeValueINT:
		return strconv.FormatInt(attr.IntVal(), 10)

	case pdata.AttributeValueMAP:
		jsonStr, _ := json.Marshal(AttributeMapToMap(attr.MapVal()))
		return string(jsonStr)

	case pdata.AttributeValueARRAY:
		jsonStr, _ := json.Marshal(attributeArrayToSlice(attr.ArrayVal()))
		return string(jsonStr)

	default:
		return fmt.Sprintf("<Unknown OpenTelemetry attribute value type %q>", attr.Type())
	}
}

// AttributeMapToMap converts an OTLP AttributeMap to a standard go map
func AttributeMapToMap(attrMap pdata.AttributeMap) map[string]interface{} {
	rawMap := make(map[string]interface{})
	attrMap.Range(func(k string, v pdata.AttributeValue) bool {
		switch v.Type() {
		case pdata.AttributeValueSTRING:
			rawMap[k] = v.StringVal()
		case pdata.AttributeValueINT:
			rawMap[k] = v.IntVal()
		case pdata.AttributeValueDOUBLE:
			rawMap[k] = v.DoubleVal()
		case pdata.AttributeValueBOOL:
			rawMap[k] = v.BoolVal()
		case pdata.AttributeValueNULL:
			rawMap[k] = nil
		case pdata.AttributeValueMAP:
			rawMap[k] = AttributeMapToMap(v.MapVal())
		case pdata.AttributeValueARRAY:
			rawMap[k] = attributeArrayToSlice(v.ArrayVal())
		}
		return true
	})
	return rawMap
}

// attributeArrayToSlice creates a slice out of a pdata.AnyValueArray.
func attributeArrayToSlice(attrArray pdata.AnyValueArray) []interface{} {
	rawSlice := make([]interface{}, 0, attrArray.Len())
	for i := 0; i < attrArray.Len(); i++ {
		v := attrArray.At(i)
		switch v.Type() {
		case pdata.AttributeValueSTRING:
			rawSlice = append(rawSlice, v.StringVal())
		case pdata.AttributeValueINT:
			rawSlice = append(rawSlice, v.IntVal())
		case pdata.AttributeValueDOUBLE:
			rawSlice = append(rawSlice, v.DoubleVal())
		case pdata.AttributeValueBOOL:
			rawSlice = append(rawSlice, v.BoolVal())
		case pdata.AttributeValueNULL:
			rawSlice = append(rawSlice, nil)
		default:
			rawSlice = append(rawSlice, "<Invalid array value>")
		}
	}
	return rawSlice
}

// DetermineValueType returns the native OTLP attribute type the string translates to.
func DetermineValueType(value string) pdata.AttributeValueType {
	for _, desc := range attrValDescriptions {
		if desc.regex.MatchString(value) {
			return desc.attrType
		}
	}
	return pdata.AttributeValueSTRING
}

// StatusCodeFromHTTP takes an HTTP status code and return the appropriate OpenTelemetry status code
// See: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/http.md#status
func StatusCodeFromHTTP(httpStatusCode int) pdata.StatusCode {
	if httpStatusCode >= 100 && httpStatusCode < 399 {
		return pdata.StatusCodeUnset
	}
	return pdata.StatusCodeError
}

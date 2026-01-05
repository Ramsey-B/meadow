package actions

import (
	anyaction "github.com/Ramsey-B/lotus/pkg/actions/any"
	"github.com/Ramsey-B/lotus/pkg/actions/array"
	"github.com/Ramsey-B/lotus/pkg/actions/date"
	"github.com/Ramsey-B/lotus/pkg/actions/number"
	"github.com/Ramsey-B/lotus/pkg/actions/object"
	"github.com/Ramsey-B/lotus/pkg/actions/registry"
	"github.com/Ramsey-B/lotus/pkg/actions/text"
	"github.com/Ramsey-B/lotus/pkg/models"
)

const (
	// Any/Conditional Action Keys
	AnyCoalesceAction     = "any_coalesce"
	AnyDefaultValueAction = "any_default"
	AnyIfElseAction       = "any_if_else"
	AnyIsNilAction        = "any_is_nil"
	AnyIsEmptyAction      = "any_is_empty"
	AnyToStringAction     = "any_to_string"

	// Array Action Keys
	ArrayContainsAction  = "array_contains"
	ArrayDistinctAction  = "array_distinct"
	ArrayFlattenAction   = "array_flatten"
	ArrayIndexOfAction   = "array_index_of"
	ArrayLengthAction    = "array_length"
	ArrayPushAction      = "array_push"
	ArrayRandomizeAction = "array_randomize"
	ArrayReverseAction   = "array_reverse"
	ArraySkipAction      = "array_skip"
	ArrayTakeAction      = "array_take"

	// Date Action Keys
	DateNowAction    = "date_now"
	DateParseAction  = "date_parse"
	DateFormatAction = "date_format"
	DateDiffAction   = "date_diff"
	DateAddAction    = "date_add"

	// Number Action Keys
	NumberAbsAction        = "number_abs"
	NumberClampAction      = "number_clamp"
	NumberParseAction      = "number_parse"
	NumberSignAction       = "number_sign"
	NumberFactorialAction  = "number_factorial"
	NumberAddAction        = "number_add"
	NumberCeilingAction    = "number_ceiling"
	NumberCubeRootAction   = "number_cube_root"
	NumberCubeAction       = "number_cube"
	NumberDivideAction     = "number_divide"
	NumberEqualsAction     = "number_equals"
	NumberExponentAction   = "number_exponent"
	NumberFloorAction      = "number_floor"
	NumberMaxAction        = "number_max"
	NumberMinAction        = "number_min"
	NumberModulusAction    = "number_modulus"
	NumberMultiplyAction   = "number_multiply"
	NumberRootAction       = "number_root"
	NumberRoundAction      = "number_round"
	NumberSquareRootAction = "number_square_root"
	NumberSquareAction     = "number_square"
	NumberSubtractAction   = "number_subtract"
	NumberToNegativeAction = "number_to_negative"
	NumberToPositiveAction = "number_to_positive"
	NumberToStringAction   = "number_to_string"
	NumberIsEvenAction     = "number_is_even"
	NumberIsOddAction      = "number_is_odd"

	// Object Action Keys
	ObjectGetAction    = "object_get"
	ObjectPickAction   = "object_pick"
	ObjectOmitAction   = "object_omit"
	ObjectMergeAction  = "object_merge"
	ObjectKeysAction   = "object_keys"
	ObjectValuesAction = "object_values"

	// Text Action Keys
	TextAllowedCharCountAction  = "text_allowed_char_count"
	TextConcatAction            = "text_concat"
	TextContainsAction          = "text_contains"
	TextEndsWithAction          = "text_ends_with"
	TextEqualsAction            = "text_equals"
	TextIndexOfAction           = "text_index_of"
	TextLengthAction            = "text_length"
	TextMaxLengthAction         = "text_max_length"
	TextMinLengthAction         = "text_min_length"
	TextPadAction               = "text_pad"
	TextReplaceAction           = "text_replace"
	TextRegexMatchAction        = "text_regex_match"
	TextRegexExtractAction      = "text_regex_extract"
	TextRegexReplaceAction      = "text_regex_replace"
	TextRequiredCharCountAction = "text_required_char_count"
	TextReverseAction           = "text_reverse"
	TextSplitAction             = "text_split"
	TextStartsWithAction        = "text_starts_with"
	TextSubstringAction         = "text_substring"
	TextToArrayAction           = "text_to_array"
	TextToBoolAction            = "text_to_bool"
	TextToLowerAction           = "text_to_lower"
	TextToNumberAction          = "text_to_number"
	TextToUpperAction           = "text_to_upper"
	TextTrimAction              = "text_trim"
)

type ActionDefinition struct {
	Key         string                   `json:"key" validate:"required"`
	Name        string                   `json:"name" validate:"required"`
	Description string                   `json:"description" validate:"required"`
	InputRules  []models.ActionInputRule `json:"input_rules" validate:"required"`
	Factory     registry.ActionFactory   `json:"-"`
}

var ActionDefinitions = map[string]ActionDefinition{
	// Any/Conditional Action Keys
	AnyCoalesceAction: {
		Key:         AnyCoalesceAction,
		Name:        "Coalesce",
		Description: "Returns the first non-nil/non-empty value from the inputs",
		InputRules:  anyaction.CoalesceRules.GetInputRules(),
		Factory:     anyaction.NewCoalesceAction,
	},
	AnyDefaultValueAction: {
		Key:         AnyDefaultValueAction,
		Name:        "Default Value",
		Description: "Returns the input value or a default if nil/empty",
		InputRules:  anyaction.DefaultValueRules.GetInputRules(),
		Factory:     anyaction.NewDefaultValueAction,
	},
	AnyIfElseAction: {
		Key:         AnyIfElseAction,
		Name:        "If-Else",
		Description: "Returns 'then' value if condition is true, otherwise 'else' value",
		InputRules:  anyaction.IfElseRules.GetInputRules(),
		Factory:     anyaction.NewIfElseAction,
	},
	AnyIsNilAction: {
		Key:         AnyIsNilAction,
		Name:        "Is Nil",
		Description: "Checks if a value is nil",
		InputRules:  anyaction.IsNilRules.GetInputRules(),
		Factory:     anyaction.NewIsNilAction,
	},
	AnyIsEmptyAction: {
		Key:         AnyIsEmptyAction,
		Name:        "Is Empty",
		Description: "Checks if a value is nil, empty string, or empty array/object",
		InputRules:  anyaction.IsEmptyRules.GetInputRules(),
		Factory:     anyaction.NewIsEmptyAction,
	},
	AnyToStringAction: {
		Key:         AnyToStringAction,
		Name:        "To String",
		Description: "Converts a value to a string",
		InputRules:  anyaction.ToStringRules.GetInputRules(),
		Factory:     anyaction.NewToStringAction,
	},

	// Array Action Keys
	ArrayContainsAction: {
		Key:         ArrayContainsAction,
		Name:        "Array Contains",
		Description: "Checks if an array contains a specific value",
		InputRules:  array.ArrayContainsRules.GetInputRules(),
		Factory:     array.NewArrayContainsAction,
	},
	ArrayDistinctAction: {
		Key:         ArrayDistinctAction,
		Name:        "Array Distinct",
		Description: "Removes duplicate values from an array",
		InputRules:  array.ArrayDistinctRules.GetInputRules(),
		Factory:     array.NewArrayDistinctAction,
	},
	ArrayIndexOfAction: {
		Key:         ArrayIndexOfAction,
		Name:        "Array Index Of",
		Description: "Finds the index of a value in an array",
		InputRules:  array.ArrayIndexOfRules.GetInputRules(),
		Factory:     array.NewArrayIndexOfAction,
	},
	ArrayLengthAction: {
		Key:         ArrayLengthAction,
		Name:        "Array Length",
		Description: "Returns the length of an array",
		InputRules:  array.ArrayLengthRules.GetInputRules(),
		Factory:     array.NewArrayLengthAction,
	},
	ArrayPushAction: {
		Key:         ArrayPushAction,
		Name:        "Array Push",
		Description: "Adds a value to the end of an array",
		InputRules:  array.ArrayPushRules.GetInputRules(),
		Factory:     array.NewArrayPushAction,
	},
	ArrayRandomizeAction: {
		Key:         ArrayRandomizeAction,
		Name:        "Array Randomize",
		Description: "Randomizes the order of an array",
		InputRules:  array.ArrayRandomizeRules.GetInputRules(),
		Factory:     array.NewArrayRandomizeAction,
	},
	ArrayReverseAction: {
		Key:         ArrayReverseAction,
		Name:        "Array Reverse",
		Description: "Reverses the order of an array",
		InputRules:  array.ArrayReverseRules.GetInputRules(),
		Factory:     array.NewArrayReverseAction,
	},

	// Date Action Keys
	DateNowAction: {
		Key:         DateNowAction,
		Name:        "Date Now",
		Description: "Returns the current date/time",
		InputRules:  date.DateNowRules.GetInputRules(),
		Factory:     date.NewDateNowAction,
	},
	DateParseAction: {
		Key:         DateParseAction,
		Name:        "Date Parse",
		Description: "Parses a date string and converts to specified format",
		InputRules:  date.DateParseRules.GetInputRules(),
		Factory:     date.NewDateParseAction,
	},
	DateFormatAction: {
		Key:         DateFormatAction,
		Name:        "Date Format",
		Description: "Formats a date string to a specified format",
		InputRules:  date.DateFormatRules.GetInputRules(),
		Factory:     date.NewDateFormatAction,
	},
	DateDiffAction: {
		Key:         DateDiffAction,
		Name:        "Date Diff",
		Description: "Calculates the difference between two dates",
		InputRules:  date.DateDiffRules.GetInputRules(),
		Factory:     date.NewDateDiffAction,
	},
	DateAddAction: {
		Key:         DateAddAction,
		Name:        "Date Add",
		Description: "Adds time to a date",
		InputRules:  date.DateAddRules.GetInputRules(),
		Factory:     date.NewDateAddAction,
	},

	// Number Action Keys
	NumberAbsAction: {
		Key:         NumberAbsAction,
		Name:        "Number Absolute",
		Description: "Returns the absolute value of a number",
		InputRules:  number.NumberAbsRules.GetInputRules(),
		Factory:     number.NewNumberAbsAction,
	},
	NumberClampAction: {
		Key:         NumberClampAction,
		Name:        "Number Clamp",
		Description: "Clamps a number between a minimum and maximum value",
		InputRules:  number.NumberClampRules.GetInputRules(),
		Factory:     number.NewNumberClampAction,
	},
	NumberParseAction: {
		Key:         NumberParseAction,
		Name:        "Number Parse",
		Description: "Parses a string to a number",
		InputRules:  number.NumberParseRules.GetInputRules(),
		Factory:     number.NewNumberParseAction,
	},
	NumberSignAction: {
		Key:         NumberSignAction,
		Name:        "Number Sign",
		Description: "Returns -1, 0, or 1 based on the sign of the number",
		InputRules:  number.NumberSignRules.GetInputRules(),
		Factory:     number.NewNumberSignAction,
	},
	NumberFactorialAction: {
		Key:         NumberFactorialAction,
		Name:        "Number Factorial",
		Description: "Calculates the factorial of a number",
		InputRules:  number.NumberFactorialRules.GetInputRules(),
		Factory:     number.NewNumberFactorialAction,
	},
	NumberAddAction: {
		Key:         NumberAddAction,
		Name:        "Number Add",
		Description: "Adds two numbers",
		InputRules:  number.NumberAddRules.GetInputRules(),
		Factory:     number.NewNumberAddAction,
	},
	NumberCeilingAction: {
		Key:         NumberCeilingAction,
		Name:        "Number Ceiling",
		Description: "Rounds a number up to the nearest integer",
		InputRules:  number.NumberCeilingRules.GetInputRules(),
		Factory:     number.NewNumberCeilingAction,
	},
	NumberCubeRootAction: {
		Key:         NumberCubeRootAction,
		Name:        "Number Cube Root",
		Description: "Calculates the cube root of a number",
		InputRules:  number.NumberCubeRootRules.GetInputRules(),
		Factory:     number.NewNumberCubeRootAction,
	},
	NumberEqualsAction: {
		Key:         NumberEqualsAction,
		Name:        "Number Equals",
		Description: "Checks if two numbers are equal",
		InputRules:  number.NumberEqualsRules.GetInputRules(),
		Factory:     number.NewNumberEqualsAction,
	},
	NumberExponentAction: {
		Key:         NumberExponentAction,
		Name:        "Number Exponent",
		Description: "Raises a number to the power of another number",
		InputRules:  number.NumberExponentRules.GetInputRules(),
		Factory:     number.NewNumberExponentAction,
	},
	NumberFloorAction: {
		Key:         NumberFloorAction,
		Name:        "Number Floor",
		Description: "Rounds a number down to the nearest integer",
		InputRules:  number.NumberFloorRules.GetInputRules(),
		Factory:     number.NewNumberFloorAction,
	},
	NumberMaxAction: {
		Key:         NumberMaxAction,
		Name:        "Number Max",
		Description: "Returns the maximum value from an array of numbers",
		InputRules:  number.NumberMaxRules.GetInputRules(),
		Factory:     number.NewNumberMaxAction,
	},
	NumberMinAction: {
		Key:         NumberMinAction,
		Name:        "Number Min",
		Description: "Returns the minimum value from an array of numbers",
		InputRules:  number.NumberMinRules.GetInputRules(),
		Factory:     number.NewNumberMinAction,
	},
	NumberModulusAction: {
		Key:         NumberModulusAction,
		Name:        "Number Modulus",
		Description: "Returns the remainder of a division",
		InputRules:  number.NumberModulusRules.GetInputRules(),
		Factory:     number.NewNumberModulusAction,
	},
	NumberMultiplyAction: {
		Key:         NumberMultiplyAction,
		Name:        "Number Multiply",
		Description: "Multiplies two numbers",
		InputRules:  number.NumberMultiplyRules.GetInputRules(),
		Factory:     number.NewNumberMultiplyAction,
	},
	NumberRootAction: {
		Key:         NumberRootAction,
		Name:        "Number Root",
		Description: "Calculates the root of a number",
		InputRules:  number.NumberRootRules.GetInputRules(),
		Factory:     number.NewNumberRootAction,
	},
	NumberRoundAction: {
		Key:         NumberRoundAction,
		Name:        "Number Round",
		Description: "Rounds a number to the nearest integer",
		InputRules:  number.NumberRoundRules.GetInputRules(),
		Factory:     number.NewNumberRoundAction,
	},
	NumberSquareRootAction: {
		Key:         NumberSquareRootAction,
		Name:        "Number Square Root",
		Description: "Calculates the square root of a number",
		InputRules:  number.NumberSquareRootRules.GetInputRules(),
		Factory:     number.NewNumberSquareRootAction,
	},
	NumberSquareAction: {
		Key:         NumberSquareAction,
		Name:        "Number Square",
		Description: "Squares a number",
		InputRules:  number.NumberSquareRules.GetInputRules(),
		Factory:     number.NewNumberSquareAction,
	},
	NumberSubtractAction: {
		Key:         NumberSubtractAction,
		Name:        "Number Subtract",
		Description: "Subtracts two numbers",
		InputRules:  number.NumberSubtractRules.GetInputRules(),
		Factory:     number.NewNumberSubtractAction,
	},
	NumberToNegativeAction: {
		Key:         NumberToNegativeAction,
		Name:        "Number To Negative",
		Description: "Converts a number to a negative number",
		InputRules:  number.NumberToNegativeRules.GetInputRules(),
		Factory:     number.NewNumberToNegativeAction,
	},
	NumberToPositiveAction: {
		Key:         NumberToPositiveAction,
		Name:        "Number To Positive",
		Description: "Converts a number to a positive number",
		InputRules:  number.NumberToPositiveRules.GetInputRules(),
		Factory:     number.NewNumberToPositiveAction,
	},
	NumberToStringAction: {
		Key:         NumberToStringAction,
		Name:        "Number To String",
		Description: "Converts a number to a string",
		InputRules:  number.NumberToStringRules.GetInputRules(),
		Factory:     number.NewNumberToStringAction,
	},
	NumberIsEvenAction: {
		Key:         NumberIsEvenAction,
		Name:        "Number Is Even",
		Description: "Checks if a number is even",
		InputRules:  number.NumberIsEvenRules.GetInputRules(),
		Factory:     number.NewNumberIsEvenAction,
	},
	NumberIsOddAction: {
		Key:         NumberIsOddAction,
		Name:        "Number Is Odd",
		Description: "Checks if a number is odd",
		InputRules:  number.NumberIsEvenRules.GetInputRules(),
		Factory:     number.NewNumberIsOddAction,
	},

	// Object Action Keys
	ObjectGetAction: {
		Key:         ObjectGetAction,
		Name:        "Object Get",
		Description: "Gets a value from an object by path",
		InputRules:  object.ObjectGetRules.GetInputRules(),
		Factory:     object.NewObjectGetAction,
	},
	ObjectPickAction: {
		Key:         ObjectPickAction,
		Name:        "Object Pick",
		Description: "Creates a new object with only the specified keys",
		InputRules:  object.ObjectPickRules.GetInputRules(),
		Factory:     object.NewObjectPickAction,
	},
	ObjectOmitAction: {
		Key:         ObjectOmitAction,
		Name:        "Object Omit",
		Description: "Creates a new object without the specified keys",
		InputRules:  object.ObjectOmitRules.GetInputRules(),
		Factory:     object.NewObjectOmitAction,
	},
	ObjectMergeAction: {
		Key:         ObjectMergeAction,
		Name:        "Object Merge",
		Description: "Merges multiple objects into one",
		InputRules:  object.ObjectMergeRules.GetInputRules(),
		Factory:     object.NewObjectMergeAction,
	},
	ObjectKeysAction: {
		Key:         ObjectKeysAction,
		Name:        "Object Keys",
		Description: "Returns an array of object keys",
		InputRules:  object.ObjectKeysRules.GetInputRules(),
		Factory:     object.NewObjectKeysAction,
	},
	ObjectValuesAction: {
		Key:         ObjectValuesAction,
		Name:        "Object Values",
		Description: "Returns an array of object values",
		InputRules:  object.ObjectValuesRules.GetInputRules(),
		Factory:     object.NewObjectValuesAction,
	},

	// Text Action Keys
	TextAllowedCharCountAction: {
		Key:         TextAllowedCharCountAction,
		Name:        "Text Allowed Char Count",
		Description: "Checks if a text contains a specific number of characters",
		InputRules:  text.TextAllowedCharCountRules.GetInputRules(),
		Factory:     text.NewTextAllowedCharCountAction,
	},
	TextConcatAction: {
		Key:         TextConcatAction,
		Name:        "Text Concat",
		Description: "Concatenates two texts",
		InputRules:  text.TextConcatRules.GetInputRules(),
		Factory:     text.NewTextConcatAction,
	},
	TextContainsAction: {
		Key:         TextContainsAction,
		Name:        "Text Contains",
		Description: "Checks if a text contains a specific substring",
		InputRules:  text.TextContainsRules.GetInputRules(),
		Factory:     text.NewTextContainsAction,
	},
	TextEndsWithAction: {
		Key:         TextEndsWithAction,
		Name:        "Text Ends With",
		Description: "Checks if a text ends with a specific substring",
		InputRules:  text.TextEndsWithRules.GetInputRules(),
		Factory:     text.NewTextEndsWithAction,
	},
	TextEqualsAction: {
		Key:         TextEqualsAction,
		Name:        "Text Equals",
		Description: "Checks if two texts are equal",
		InputRules:  text.TextEqualsRules.GetInputRules(),
		Factory:     text.NewTextEqualsAction,
	},
	TextIndexOfAction: {
		Key:         TextIndexOfAction,
		Name:        "Text Index Of",
		Description: "Finds the index of a substring in a text",
		InputRules:  text.TextIndexOfRules.GetInputRules(),
		Factory:     text.NewTextLengthAction,
	},
	TextMaxLengthAction: {
		Key:         TextMaxLengthAction,
		Name:        "Text Max Length",
		Description: "Checks if a text is longer than a specific number of characters",
		InputRules:  text.TextMaxLengthRules.GetInputRules(),
		Factory:     text.NewTextReplaceAction,
	},
	TextRequiredCharCountAction: {
		Key:         TextRequiredCharCountAction,
		Name:        "Text Required Char Count",
		Description: "Checks if a text contains a specific number of characters",
		InputRules:  text.TextRequiredCharCountRules.GetInputRules(),
		Factory:     text.NewTextRequiredCharCountAction,
	},
	TextReverseAction: {
		Key:         TextReverseAction,
		Name:        "Text Reverse",
		Description: "Reverses a text",
		InputRules:  text.TextReverseRules.GetInputRules(),
		Factory:     text.NewTextReverseAction,
	},
	TextSplitAction: {
		Key:         TextSplitAction,
		Name:        "Text Split",
		Description: "Splits a text into an array of substrings",
		InputRules:  text.TextSplitRules.GetInputRules(),
		Factory:     text.NewTextSplitAction,
	},
	TextStartsWithAction: {
		Key:         TextStartsWithAction,
		Name:        "Text Starts With",
		Description: "Checks if a text starts with a specific substring",
		InputRules:  text.TextStartsWithRules.GetInputRules(),
		Factory:     text.NewTextStartsWithAction,
	},
	TextToArrayAction: {
		Key:         TextToArrayAction,
		Name:        "Text To Array",
		Description: "Converts a text to an array of characters",
		InputRules:  text.TextToArrayRules.GetInputRules(),
		Factory:     text.NewTextToArrayAction,
	},
	TextToBoolAction: {
		Key:         TextToBoolAction,
		Name:        "Text To Bool",
		Description: "Converts a text to a boolean value",
		InputRules:  text.TextToBoolRules.GetInputRules(),
		Factory:     text.NewTextToBoolAction,
	},
	TextToLowerAction: {
		Key:         TextToLowerAction,
		Name:        "Text To Lower",
		Description: "Converts a text to lowercase",
		InputRules:  text.TextToLowerRules.GetInputRules(),
		Factory:     text.NewTextToLowerAction,
	},
	TextToNumberAction: {
		Key:         TextToNumberAction,
		Name:        "Text To Number",
		Description: "Converts a text to a number",
		InputRules:  text.TextToNumberRules.GetInputRules(),
		Factory:     text.NewTextToNumberAction,
	},
	TextMinLengthAction: {
		Key:         TextMinLengthAction,
		Name:        "Text Min Length",
		Description: "Checks if a text is shorter than a specific number of characters",
		InputRules:  text.TextMinLengthRules.GetInputRules(),
		Factory:     text.NewTextMinLengthAction,
	},
	TextReplaceAction: {
		Key:         TextReplaceAction,
		Name:        "Text Replace",
		Description: "Replaces a substring in a text",
		InputRules:  text.TextReplaceRules.GetInputRules(),
		Factory:     text.NewTextReplaceAction,
	},
	TextToUpperAction: {
		Key:         TextToUpperAction,
		Name:        "Text To Upper",
		Description: "Converts a text to uppercase",
		InputRules:  text.TextToUpperRules.GetInputRules(),
		Factory:     text.NewTextToUpperAction,
	},
	TextTrimAction: {
		Key:         TextTrimAction,
		Name:        "Text Trim",
		Description: "Removes whitespace or specified characters from text",
		InputRules:  text.TextTrimRules.GetInputRules(),
		Factory:     text.NewTextTrimAction,
	},
	TextPadAction: {
		Key:         TextPadAction,
		Name:        "Text Pad",
		Description: "Pads text to a specified length",
		InputRules:  text.TextPadRules.GetInputRules(),
		Factory:     text.NewTextPadAction,
	},
	TextSubstringAction: {
		Key:         TextSubstringAction,
		Name:        "Text Substring",
		Description: "Extracts a substring from text",
		InputRules:  text.TextSubstringRules.GetInputRules(),
		Factory:     text.NewTextSubstringAction,
	},
	TextRegexMatchAction: {
		Key:         TextRegexMatchAction,
		Name:        "Text Regex Match",
		Description: "Checks if text matches a regular expression",
		InputRules:  text.TextRegexMatchRules.GetInputRules(),
		Factory:     text.NewTextRegexMatchAction,
	},
	TextRegexExtractAction: {
		Key:         TextRegexExtractAction,
		Name:        "Text Regex Extract",
		Description: "Extracts matches from text using a regular expression",
		InputRules:  text.TextRegexExtractRules.GetInputRules(),
		Factory:     text.NewTextRegexExtractAction,
	},
	TextRegexReplaceAction: {
		Key:         TextRegexReplaceAction,
		Name:        "Text Regex Replace",
		Description: "Replaces matches in text using a regular expression",
		InputRules:  text.TextRegexReplaceRules.GetInputRules(),
		Factory:     text.NewTextRegexReplaceAction,
	},
}

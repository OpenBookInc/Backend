package tester_common

import (
	"strconv"

	common "matching-clients/src/gen"
	"github.com/openbook/shared/utils"
)

// ParseLegsFromForm parses leg security IDs and over/under from form values
func ParseLegsFromForm(legIdsStr, isOversStr string) ([]Leg, []*common.UUID) {
	var trackerLegs []Leg
	var legUUIDs []*common.UUID
	if legIdsStr == "" || isOversStr == "" {
		return trackerLegs, legUUIDs
	}
	legIdStrs := SplitAndTrim(legIdsStr, ",")
	isOverStrs := SplitAndTrim(isOversStr, ",")
	for i := 0; i < len(legIdStrs) && i < len(isOverStrs); i++ {
		var legUUID *common.UUID
		if parsed, err := utils.ParseUUID(legIdStrs[i]); err == nil {
			legUUID = &common.UUID{Upper: parsed.Upper(), Lower: parsed.Lower()}
		} else {
			legId, _ := strconv.ParseUint(legIdStrs[i], 10, 64)
			legUUID = &common.UUID{Upper: 0, Lower: legId}
		}
		isOver := isOverStrs[i] == "true" || isOverStrs[i] == "1"
		trackerLegs = append(trackerLegs, Leg{
			LegSecurityID: utils.UUIDFromUint64(legUUID.Upper, legUUID.Lower),
			IsOver:        isOver,
		})
		legUUIDs = append(legUUIDs, legUUID)
	}
	return trackerLegs, legUUIDs
}

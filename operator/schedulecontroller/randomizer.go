package schedulecontroller

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/big"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

const (
	ScheduleHourlyRandom   = "@hourly-random"
	ScheduleDailyRandom    = "@daily-random"
	ScheduleYearlyRandom   = "@yearly-random"
	ScheduleAnnuallyRandom = "@annually-random"
	ScheduleMonthlyRandom  = "@monthly-random"
	ScheduleWeeklyRandom   = "@weekly-random"
)

func createSeed(schedule *k8upv1.Schedule, jobType k8upv1.JobType) string {
	return schedule.Namespace + "/" + schedule.Name + "@" + jobType.String()
}

// randomizeSchedule randomizes the given originalSchedule with a seed. The originalSchedule has to be one of the supported
// '@x-random' predefined schedules, otherwise it returns an error with the original schedule unmodified.
func randomizeSchedule(seed string, originalSchedule k8upv1.ScheduleDefinition) (k8upv1.ScheduleDefinition, error) {
	checksum := calculateChecksumFromSeed(seed)

	minute := remainderFromModulo(checksum, 60, 0)
	hour := remainderFromModulo(checksum, 24, 0)
	// A month can have between 27 and 31 days. Cron does not automatically schedule at the end of the month if 31 in February.
	// To not cause a skip of a scheduled backup that could potentially raise alerts, we cap the day-of-month to 27 so it fits in all months.
	dayOfMonth := remainderFromModulo(checksum, 27, 1)

	switch originalSchedule {
	case ScheduleHourlyRandom:
		return k8upv1.ScheduleDefinition(fmt.Sprintf("%d * * * *", minute)), nil
	case ScheduleDailyRandom:
		return k8upv1.ScheduleDefinition(fmt.Sprintf("%d %d * * *", minute, hour)), nil
	case ScheduleMonthlyRandom:
		return k8upv1.ScheduleDefinition(fmt.Sprintf("%d %d %d * *", minute, hour, dayOfMonth)), nil
	case ScheduleAnnuallyRandom:
	case ScheduleYearlyRandom:
		month := remainderFromModulo(checksum, 12, 1)
		return k8upv1.ScheduleDefinition(fmt.Sprintf("%d %d %d %d *", minute, hour, dayOfMonth, month)), nil
	case ScheduleWeeklyRandom:
		weekday := remainderFromModulo(checksum, 6, 0)
		return k8upv1.ScheduleDefinition(fmt.Sprintf("%d %d * * %d", minute, hour, weekday)), nil
	default:
		return originalSchedule, fmt.Errorf("unrecognized random schedule: '%s'", originalSchedule)
	}
	return originalSchedule, nil
}

// calculateChecksumFromSeed calculates a SHA1 hexadecimal checksum from the given seed.
func calculateChecksumFromSeed(seed string) *big.Int {
	hash := sha1.New()
	hash.Write([]byte(seed))
	sum := hex.EncodeToString(hash.Sum(nil))
	sumBase16, _ := new(big.Int).SetString(sum, 16)
	return sumBase16
}

// remainderFromModulo calculates the remainder from (dividend % divisor), then adds the offset.
func remainderFromModulo(dividend *big.Int, divisor, offset int64) *big.Int {
	return new(big.Int).Add(big.NewInt(offset), new(big.Int).Mod(dividend, big.NewInt(divisor)))
}

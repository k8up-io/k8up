package jobcontroller

import batchv1 "k8s.io/api/batch/v1"

// FindStatusCondition finds the condition with the given type in the batchv1.JobCondition slice.
// Returns nil if not found.
func FindStatusCondition(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) *batchv1.JobCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

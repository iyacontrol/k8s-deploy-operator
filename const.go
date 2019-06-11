package main


const (
	K8sDeployStageInitialization = iota
	K8sDeployStageCanary
	K8sDeployStageRollBack
	K8sDeployStageRollup
)


package natun

// Since controller-tools cannot scan internal packages, we're specifying here all the RBAC markers

// Certs
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers;certificates,namespace=natun-system,verbs=get;create;update;patch;delete;watch;list
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;update;patch;watch;list
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;update;patch;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;patch;list;watch,namespace=natun-system

// Stats
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;watch;get

// Operator Controllers
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features/finalizers,verbs=update

// Engine Controllers
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features,verbs=get;list;watch
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors,verbs=get;list;watch

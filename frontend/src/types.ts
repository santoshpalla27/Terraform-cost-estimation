// Extended types for cost explanation UX

export interface EstimationResult {
    totalCost: number
    confidence: number
    coverage: CoverageStats
    resources: ResourceCost[]
    symbolicReasons: Record<string, string[]>
    unsupportedTypes: string[]
    pricingSnapshot?: SnapshotInfo
}

export interface CoverageStats {
    numericPercent: number
    symbolicPercent: number
    indirectPercent: number
    unsupportedPercent: number
}

export interface ResourceCost {
    address: string
    type: string
    cost: number
    confidence: number
    coverageType: 'numeric' | 'symbolic' | 'indirect' | 'unsupported'
    explanation?: CostExplanation
    components?: CostComponent[]
    lineage?: CostLineage
}

export interface CostComponent {
    name: string
    category: string
    cost: number
    unit: string
    quantity: number
    rate: number
    isSymbolic: boolean
    symbolicReason?: string
}

export interface CostExplanation {
    formula: string
    inputs: { name: string; value: string; source: string }[]
    isSymbolic: boolean
    symbolicReason?: string
    confidenceCeiling: number
}

export interface CostLineage {
    rateKey: RateKey
    snapshotId: string
    source: string
    resolvedAt: string
}

export interface RateKey {
    cloud: string
    service: string
    productFamily: string
    region: string
    attributes: Record<string, string>
}

export interface SnapshotInfo {
    id: string
    cloud: string
    region: string
    alias: string
    fetchedAt: string
    source: string
}

// Diff types
export interface DiffResult {
    oldCost: number
    newCost: number
    delta: number
    deltaPercent: number
    oldConfidence: number
    newConfidence: number
    changes: ResourceChange[]
    summary: DiffSummary
}

export interface ResourceChange {
    address: string
    type: string
    changeType: 'create' | 'destroy' | 'update' | 'no_change'
    oldCost: number
    newCost: number
    delta: number
    narrative: string
    drivers: CostDriver[]
}

export interface CostDriver {
    attribute: string
    oldValue: string
    newValue: string
    impact: number
}

export interface DiffSummary {
    createdCount: number
    destroyedCount: number
    updatedCount: number
    topIncreases: ResourceChange[]
    topDecreases: ResourceChange[]
    symbolicChanges: string[]
}

// Policy types
export interface PolicyResult {
    passed: boolean
    violations: PolicyViolation[]
    warnings: PolicyWarning[]
}

export interface PolicyViolation {
    rule: string
    message: string
    severity: 'error' | 'warning'
    resource?: string
    threshold?: number
    actual?: number
}

export interface PolicyWarning {
    rule: string
    message: string
    resource?: string
}

// CI types
export interface CIReport {
    estimation: EstimationResult
    diff?: DiffResult
    policy?: PolicyResult
    mode: 'informational' | 'warning' | 'blocking' | 'strict'
    checkConclusion: 'success' | 'neutral' | 'failure'
}

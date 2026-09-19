package main

import (
	"sort"
	"strings"

	arxiv "github.com/lantos1618/arxiv.gg"
)

// Category labels verified on 2026-09-19 against the official taxonomy:
// https://arxiv.org/category_taxonomy
// Legacy names and discipline assignments come from arXiv's taxonomy source:
// https://github.com/arXiv/arxiv-base/blob/develop/arxiv/taxonomy/definitions.py
// Keep legacy codes distinct: their counts and links refer to the stored code,
// not to the category that later subsumed it.
type categoryDefinition struct {
	Name       string
	Discipline string
	Legacy     bool
}

var categoryDefinitions = map[string]categoryDefinition{
	"acc-phys":           {"Accelerator Physics", "Physics", true},
	"adap-org":           {"Adaptation, Noise, and Self-Organizing Systems", "Physics", true},
	"alg-geom":           {"Algebraic Geometry", "Mathematics", true},
	"ao-sci":             {"Atmospheric-Oceanic Sciences", "Physics", true},
	"astro-ph":           {"Astrophysics", "Physics", true},
	"astro-ph.CO":        {"Cosmology and Nongalactic Astrophysics", "Physics", false},
	"astro-ph.EP":        {"Earth and Planetary Astrophysics", "Physics", false},
	"astro-ph.GA":        {"Astrophysics of Galaxies", "Physics", false},
	"astro-ph.HE":        {"High Energy Astrophysical Phenomena", "Physics", false},
	"astro-ph.IM":        {"Instrumentation and Methods for Astrophysics", "Physics", false},
	"astro-ph.SR":        {"Solar and Stellar Astrophysics", "Physics", false},
	"atom-ph":            {"Atomic, Molecular and Optical Physics", "Physics", true},
	"bayes-an":           {"Bayesian Analysis", "Physics", true},
	"chao-dyn":           {"Chaotic Dynamics", "Physics", true},
	"chem-ph":            {"Chemical Physics", "Physics", true},
	"cmp-lg":             {"Computation and Language", "Computer Science", true},
	"comp-gas":           {"Cellular Automata and Lattice Gases", "Physics", true},
	"cond-mat":           {"Condensed Matter", "Physics", true},
	"cond-mat.dis-nn":    {"Disordered Systems and Neural Networks", "Physics", false},
	"cond-mat.mes-hall":  {"Mesoscale and Nanoscale Physics", "Physics", false},
	"cond-mat.mtrl-sci":  {"Materials Science", "Physics", false},
	"cond-mat.other":     {"Other Condensed Matter", "Physics", false},
	"cond-mat.quant-gas": {"Quantum Gases", "Physics", false},
	"cond-mat.soft":      {"Soft Condensed Matter", "Physics", false},
	"cond-mat.stat-mech": {"Statistical Mechanics", "Physics", false},
	"cond-mat.str-el":    {"Strongly Correlated Electrons", "Physics", false},
	"cond-mat.supr-con":  {"Superconductivity", "Physics", false},
	"cs.AI":              {"Artificial Intelligence", "Computer Science", false},
	"cs.AR":              {"Hardware Architecture", "Computer Science", false},
	"cs.CC":              {"Computational Complexity", "Computer Science", false},
	"cs.CE":              {"Computational Engineering, Finance, and Science", "Computer Science", false},
	"cs.CG":              {"Computational Geometry", "Computer Science", false},
	"cs.CL":              {"Computation and Language", "Computer Science", false},
	"cs.CR":              {"Cryptography and Security", "Computer Science", false},
	"cs.CV":              {"Computer Vision and Pattern Recognition", "Computer Science", false},
	"cs.CY":              {"Computers and Society", "Computer Science", false},
	"cs.DB":              {"Databases", "Computer Science", false},
	"cs.DC":              {"Distributed, Parallel, and Cluster Computing", "Computer Science", false},
	"cs.DL":              {"Digital Libraries", "Computer Science", false},
	"cs.DM":              {"Discrete Mathematics", "Computer Science", false},
	"cs.DS":              {"Data Structures and Algorithms", "Computer Science", false},
	"cs.ET":              {"Emerging Technologies", "Computer Science", false},
	"cs.FL":              {"Formal Languages and Automata Theory", "Computer Science", false},
	"cs.GL":              {"General Literature", "Computer Science", false},
	"cs.GR":              {"Graphics", "Computer Science", false},
	"cs.GT":              {"Computer Science and Game Theory", "Computer Science", false},
	"cs.HC":              {"Human-Computer Interaction", "Computer Science", false},
	"cs.IR":              {"Information Retrieval", "Computer Science", false},
	"cs.IT":              {"Information Theory", "Computer Science", false},
	"cs.LG":              {"Machine Learning", "Computer Science", false},
	"cs.LO":              {"Logic in Computer Science", "Computer Science", false},
	"cs.MA":              {"Multiagent Systems", "Computer Science", false},
	"cs.MM":              {"Multimedia", "Computer Science", false},
	"cs.MS":              {"Mathematical Software", "Computer Science", false},
	"cs.NA":              {"Numerical Analysis", "Computer Science", false},
	"cs.NE":              {"Neural and Evolutionary Computing", "Computer Science", false},
	"cs.NI":              {"Networking and Internet Architecture", "Computer Science", false},
	"cs.OH":              {"Other Computer Science", "Computer Science", false},
	"cs.OS":              {"Operating Systems", "Computer Science", false},
	"cs.PF":              {"Performance", "Computer Science", false},
	"cs.PL":              {"Programming Languages", "Computer Science", false},
	"cs.RO":              {"Robotics", "Computer Science", false},
	"cs.SC":              {"Symbolic Computation", "Computer Science", false},
	"cs.SD":              {"Sound", "Computer Science", false},
	"cs.SE":              {"Software Engineering", "Computer Science", false},
	"cs.SI":              {"Social and Information Networks", "Computer Science", false},
	"cs.SY":              {"Systems and Control", "Computer Science", false},
	"dg-ga":              {"Differential Geometry", "Mathematics", true},
	"econ.EM":            {"Econometrics", "Economics", false},
	"econ.GN":            {"General Economics", "Economics", false},
	"econ.TH":            {"Theoretical Economics", "Economics", false},
	"eess.AS":            {"Audio and Speech Processing", "Electrical Engineering and Systems Science", false},
	"eess.IV":            {"Image and Video Processing", "Electrical Engineering and Systems Science", false},
	"eess.SP":            {"Signal Processing", "Electrical Engineering and Systems Science", false},
	"eess.SY":            {"Systems and Control", "Electrical Engineering and Systems Science", false},
	"funct-an":           {"Functional Analysis", "Mathematics", true},
	"gr-qc":              {"General Relativity and Quantum Cosmology", "Physics", false},
	"hep-ex":             {"High Energy Physics - Experiment", "Physics", false},
	"hep-lat":            {"High Energy Physics - Lattice", "Physics", false},
	"hep-ph":             {"High Energy Physics - Phenomenology", "Physics", false},
	"hep-th":             {"High Energy Physics - Theory", "Physics", false},
	"math-ph":            {"Mathematical Physics", "Physics", false},
	"math.AC":            {"Commutative Algebra", "Mathematics", false},
	"math.AG":            {"Algebraic Geometry", "Mathematics", false},
	"math.AP":            {"Analysis of PDEs", "Mathematics", false},
	"math.AT":            {"Algebraic Topology", "Mathematics", false},
	"math.CA":            {"Classical Analysis and ODEs", "Mathematics", false},
	"math.CO":            {"Combinatorics", "Mathematics", false},
	"math.CT":            {"Category Theory", "Mathematics", false},
	"math.CV":            {"Complex Variables", "Mathematics", false},
	"math.DG":            {"Differential Geometry", "Mathematics", false},
	"math.DS":            {"Dynamical Systems", "Mathematics", false},
	"math.FA":            {"Functional Analysis", "Mathematics", false},
	"math.GM":            {"General Mathematics", "Mathematics", false},
	"math.GN":            {"General Topology", "Mathematics", false},
	"math.GR":            {"Group Theory", "Mathematics", false},
	"math.GT":            {"Geometric Topology", "Mathematics", false},
	"math.HO":            {"History and Overview", "Mathematics", false},
	"math.IT":            {"Information Theory", "Mathematics", false},
	"math.KT":            {"K-Theory and Homology", "Mathematics", false},
	"math.LO":            {"Logic", "Mathematics", false},
	"math.MG":            {"Metric Geometry", "Mathematics", false},
	"math.MP":            {"Mathematical Physics", "Mathematics", false},
	"math.NA":            {"Numerical Analysis", "Mathematics", false},
	"math.NT":            {"Number Theory", "Mathematics", false},
	"math.OA":            {"Operator Algebras", "Mathematics", false},
	"math.OC":            {"Optimization and Control", "Mathematics", false},
	"math.PR":            {"Probability", "Mathematics", false},
	"math.QA":            {"Quantum Algebra", "Mathematics", false},
	"math.RA":            {"Rings and Algebras", "Mathematics", false},
	"math.RT":            {"Representation Theory", "Mathematics", false},
	"math.SG":            {"Symplectic Geometry", "Mathematics", false},
	"math.SP":            {"Spectral Theory", "Mathematics", false},
	"math.ST":            {"Statistics Theory", "Mathematics", false},
	"mtrl-th":            {"Materials Theory", "Physics", true},
	"nlin.AO":            {"Adaptation and Self-Organizing Systems", "Physics", false},
	"nlin.CD":            {"Chaotic Dynamics", "Physics", false},
	"nlin.CG":            {"Cellular Automata and Lattice Gases", "Physics", false},
	"nlin.PS":            {"Pattern Formation and Solitons", "Physics", false},
	"nlin.SI":            {"Exactly Solvable and Integrable Systems", "Physics", false},
	"nucl-ex":            {"Nuclear Experiment", "Physics", false},
	"nucl-th":            {"Nuclear Theory", "Physics", false},
	"patt-sol":           {"Pattern Formation and Solitons", "Physics", true},
	"physics.acc-ph":     {"Accelerator Physics", "Physics", false},
	"physics.ao-ph":      {"Atmospheric and Oceanic Physics", "Physics", false},
	"physics.app-ph":     {"Applied Physics", "Physics", false},
	"physics.atm-clus":   {"Atomic and Molecular Clusters", "Physics", false},
	"physics.atom-ph":    {"Atomic Physics", "Physics", false},
	"physics.bio-ph":     {"Biological Physics", "Physics", false},
	"physics.chem-ph":    {"Chemical Physics", "Physics", false},
	"physics.class-ph":   {"Classical Physics", "Physics", false},
	"physics.comp-ph":    {"Computational Physics", "Physics", false},
	"physics.data-an":    {"Data Analysis, Statistics and Probability", "Physics", false},
	"physics.ed-ph":      {"Physics Education", "Physics", false},
	"physics.flu-dyn":    {"Fluid Dynamics", "Physics", false},
	"physics.gen-ph":     {"General Physics", "Physics", false},
	"physics.geo-ph":     {"Geophysics", "Physics", false},
	"physics.hist-ph":    {"History and Philosophy of Physics", "Physics", false},
	"physics.ins-det":    {"Instrumentation and Detectors", "Physics", false},
	"physics.med-ph":     {"Medical Physics", "Physics", false},
	"physics.optics":     {"Optics", "Physics", false},
	"physics.plasm-ph":   {"Plasma Physics", "Physics", false},
	"physics.pop-ph":     {"Popular Physics", "Physics", false},
	"physics.soc-ph":     {"Physics and Society", "Physics", false},
	"physics.space-ph":   {"Space Physics", "Physics", false},
	"plasm-ph":           {"Plasma Physics", "Physics", true},
	"q-alg":              {"Quantum Algebra and Topology", "Mathematics", true},
	"q-bio":              {"Quantitative Biology", "Quantitative Biology", true},
	"q-bio.BM":           {"Biomolecules", "Quantitative Biology", false},
	"q-bio.CB":           {"Cell Behavior", "Quantitative Biology", false},
	"q-bio.GN":           {"Genomics", "Quantitative Biology", false},
	"q-bio.MN":           {"Molecular Networks", "Quantitative Biology", false},
	"q-bio.NC":           {"Neurons and Cognition", "Quantitative Biology", false},
	"q-bio.OT":           {"Other Quantitative Biology", "Quantitative Biology", false},
	"q-bio.PE":           {"Populations and Evolution", "Quantitative Biology", false},
	"q-bio.QM":           {"Quantitative Methods", "Quantitative Biology", false},
	"q-bio.SC":           {"Subcellular Processes", "Quantitative Biology", false},
	"q-bio.TO":           {"Tissues and Organs", "Quantitative Biology", false},
	"q-fin.CP":           {"Computational Finance", "Quantitative Finance", false},
	"q-fin.EC":           {"Economics", "Quantitative Finance", false},
	"q-fin.GN":           {"General Finance", "Quantitative Finance", false},
	"q-fin.MF":           {"Mathematical Finance", "Quantitative Finance", false},
	"q-fin.PM":           {"Portfolio Management", "Quantitative Finance", false},
	"q-fin.PR":           {"Pricing of Securities", "Quantitative Finance", false},
	"q-fin.RM":           {"Risk Management", "Quantitative Finance", false},
	"q-fin.ST":           {"Statistical Finance", "Quantitative Finance", false},
	"q-fin.TR":           {"Trading and Market Microstructure", "Quantitative Finance", false},
	"quant-ph":           {"Quantum Physics", "Physics", false},
	"solv-int":           {"Exactly Solvable and Integrable Systems", "Physics", true},
	"stat.AP":            {"Applications", "Statistics", false},
	"stat.CO":            {"Computation", "Statistics", false},
	"stat.ME":            {"Methodology", "Statistics", false},
	"stat.ML":            {"Machine Learning", "Statistics", false},
	"stat.OT":            {"Other Statistics", "Statistics", false},
	"stat.TH":            {"Statistics Theory", "Statistics", false},
	"supr-con":           {"Superconductivity", "Physics", true},
}

// Archive names support aggregate category routes without guessing subject names.
var categoryArchiveDefinitions = map[string]categoryDefinition{
	"cs":      {"Computer Science", "Computer Science", false},
	"econ":    {"Economics", "Economics", false},
	"eess":    {"Electrical Engineering and Systems Science", "Electrical Engineering and Systems Science", false},
	"math":    {"Mathematics", "Mathematics", false},
	"nlin":    {"Nonlinear Sciences", "Physics", false},
	"physics": {"Physics", "Physics", false},
	"q-fin":   {"Quantitative Finance", "Quantitative Finance", false},
	"stat":    {"Statistics", "Statistics", false},
}

func categoryDefinitionFor(code string) categoryDefinition {
	if definition, ok := categoryDefinitions[code]; ok {
		return definition
	}
	if definition, ok := categoryArchiveDefinitions[code]; ok {
		return definition
	}
	// Preserve unfamiliar codes visibly instead of inventing a subject name.
	return categoryDefinition{Name: code, Discipline: "Other subjects"}
}

func categoryDisplayName(code string) string {
	definition := categoryDefinitionFor(code)
	if definition.Name == code {
		return code
	}
	// Existing metadata sentences use lower-case subject names.
	return strings.ToLower(definition.Name)
}

type categoryDirectoryItem struct {
	Code   string
	Name   string
	Count  int64
	Legacy bool
}

type categoryDirectoryGroup struct {
	ID         string
	Name       string
	Categories []categoryDirectoryItem
}

func groupCategories(categories []arxiv.CategoryCount) []categoryDirectoryGroup {
	byDiscipline := make(map[string][]categoryDirectoryItem)
	for _, category := range categories {
		definition := categoryDefinitionFor(category.Name)
		byDiscipline[definition.Discipline] = append(byDiscipline[definition.Discipline], categoryDirectoryItem{
			Code: category.Name, Name: definition.Name, Count: int64(category.Count), Legacy: definition.Legacy,
		})
	}
	groups := make([]categoryDirectoryGroup, 0, len(byDiscipline))
	for discipline, items := range byDiscipline {
		sort.Slice(items, func(i, j int) bool {
			if items[i].Name == items[j].Name {
				return items[i].Code < items[j].Code
			}
			return items[i].Name < items[j].Name
		})
		groups = append(groups, categoryDirectoryGroup{
			ID:   "discipline-" + strings.ReplaceAll(strings.ToLower(discipline), " ", "-"),
			Name: discipline, Categories: items,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Name == "Other subjects" {
			return false
		}
		if groups[j].Name == "Other subjects" {
			return true
		}
		return groups[i].Name < groups[j].Name
	})
	return groups
}

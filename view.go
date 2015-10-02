package evo

import (
	"fmt"
	"math"
	"sync"
)

// Views are cached for reuse in this pool.
// Reusing views reduces the chance that a new genome slice is allocated.
var pool = sync.Pool{
	New: func() interface{} {
		var v View
		return v
	},
}

// Views are static collections of genomes. The usual way to obtain a view is
// by calling the View() method of a population; the collection returned is a
// "snapshot" of the genomes in the population. In the case of a hybrid
// population (one where the members are themselves populations), a view
// captures the genomes of the sub-populations.
//
// Views provide statistics on the genomes they contain. The usual way to gather
// statistics on a population is to create a view.
type View struct {
	members  []Genome
	max, min int     // indexes of the max/min genomes
	mean     float64 // average fitness of all members
	m2       float64 // sum of squares of deviation from the mean
	len      float64 // len(v.members) as a float64
}

// NewView creates a view containing the genomes passed as arguments. If a
// population is passed, the view contains the members of the population rather
// than the population itself. Thus `NewView(myPopulation)` is equivalent to
// `myPopulation.View()`
func NewView(subs ...Genome) View {
	var (
		v      = pool.Get().(View)
		maxfit = math.Inf(-1)
		minfit = math.Inf(+1)
	)

	// The size of the view will be at least len(subs).
	if cap(v.members) < len(subs) {
		v.members = make([]Genome, 0, len(subs))
	}

	// We calculate the mean and variance during construction so that calls to
	// the statistics methods take constant time. For each argument passed, we
	// have two cases:
	//
	// The base case is that the argument is an atomic genome. We can simply add
	// the argument to the view and update the statistics using Knuth's
	// algorithm for computing variance [1].
	//
	// The recursive case is that the argument is a population. We get a subview
	// of the population and merge it into this view using a pair-wise algorithm
	// for computing variance from Chan et al. [2] (of which Knuth's algorithm
	// is a special case).
	//
	// See https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance
	//
	// [1]: Donald E. Knuth (1998). The Art of Computer Programming, volume 2:
	//      Seminumerical Algorithms, 3rd edn., p. 232. Boston: Addison-Wesley.
	//
	// [2]: Chan, Tony F.; Golub, Gene H.; LeVeque, Randall J. (1983).
	//      Algorithms for Computing the Sample Variance: Analysis and
	//      Recommendations. The American Statistician 37, 242-247.
	//      http://www.jstor.org/stable/2683386
	for i := range subs {
		switch sub := subs[i].(type) {

		case Population:
			subview := sub.View()
			delta := subview.mean - v.mean
			newlen := subview.len + v.len

			// max
			submaxfit := subview.Max().Fitness()
			if submaxfit > maxfit {
				v.max = len(v.members) + subview.max
				maxfit = submaxfit
			}

			// min
			subminfit := subview.Min().Fitness()
			if subminfit < minfit {
				v.min = len(v.members) + subview.min
				minfit = subminfit
			}

			// mean
			v.mean += delta * (subview.len / newlen)

			// sum of squares
			v.m2 += subview.m2
			v.m2 += delta * delta * (subview.len * v.len / newlen)

			// len
			v.len = newlen
			v.members = append(v.members, subview.members...)

			subview.Close()

		default:
			subfit := sub.Fitness()
			delta := subfit - v.mean
			newlen := v.len + 1

			// max
			if subfit > maxfit {
				v.max = len(v.members)
				maxfit = subfit
			}

			// min
			if subfit < minfit {
				v.min = len(v.members)
				minfit = subfit
			}

			// mean
			v.mean += delta / newlen

			// sum of squares
			v.m2 += delta * delta * (v.len / newlen)

			// len
			v.len = newlen
			v.members = append(v.members, sub)
		}
	}

	return v
}

// Close allows resources controlled by a view to be reused by the system,
// reducing the allocation cost when constructing a view. It is not safe to use
// a view after it is closed.
// TODO: panic if a view is used after being closed
func (v View) Close() {
	v.members = v.members[0:0]
	v.max, v.min = 0, 0
	v.mean, v.m2, v.len = 0, 0, 0
	pool.Put(v)
}

// Members returns the genomes in the view.
func (v View) Members() []Genome {
	return v.members
}

// Max returns the genome with the best fitness.
func (v View) Max() Genome {
	return v.members[v.max]
}

// Min returns the genome with the worst fitness.
func (v View) Min() Genome {
	return v.members[v.min]
}

// Range returns the difference in the maximum and minimum fitness.
func (v View) Range() float64 {
	return v.Max().Fitness() - v.Min().Fitness()
}

// Mean returns the average fitness.
func (v View) Mean() float64 {
	return v.mean
}

// Variance returns the population variance of fitness.
func (v View) Variance() float64 {
	return v.m2 / v.len
}

// StdDeviation returns the population standard deviation of fitness.
func (v View) StdDeviation() float64 {
	return math.Sqrt(v.m2 / v.len)
}

// Len returns the number of genomes in the view.
func (v View) Len() int {
	return len(v.members)
}

// String returns a string listing a summary of the statistics.
func (v View) String() string {
	return fmt.Sprintf("Max: %f | Min: %f | SD: %f",
		v.Max().Fitness(),
		v.Min().Fitness(),
		v.StdDeviation())
}

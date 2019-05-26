/*
Package jump implements the "jump consistent hash" algorithm.

Example

 h := jump.Hash(256, 1024)  // h = 520

Reference C++ implementation[1]

 int32_t JumpConsistentHash(uint64_t key, int32_t num_buckets) {
   int64_t b = -1, j = 0;
   while (j < num_buckets) {
     b = j;
     key = key * 2862933555777941757ULL + 1;
     j = (b + 1) * (double(1LL << 31) / double((key >> 33) + 1));
   }
   return b;
 }

Explanation of the algorithm

Jump consistent hash works by computing when its output changes as the
number of buckets increases. Let ch(key, num_buckets) be the consistent hash
for the key when there are num_buckets buckets. Clearly, for any key, k,
ch(k, 1) is 0, since there is only the one bucket. In order for the
consistent hash function to balanced, ch(k, 2) will have to stay at 0 for
half the keys, k, while it will have to jump to 1 for the other half. In
general, ch(k, n+1) has to stay the same as ch(k, n) for n/(n+1) of the
keys, and jump to n for the other 1/(n+1) of the keys.

Here are examples of the consistent hash values for three keys, k1, k2, and
k3, as num_buckets goes up:

     │ 1 │ 2 │ 3 │ 4 │ 5 │ 6 │ 7 │ 8 │ 9 │ 10 │ 11 │ 12 │ 13 │ 14
  ───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼────┼────┼────┼────┼────
  k1 │ 0 │ 0 │ 2 │ 2 │ 4 │ 4 │ 4 │ 4 │ 4 │  4 │  4 │  4 │  4 │  4
  ───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼────┼────┼────┼────┼────
  k2 │ 0 │ 1 │ 1 │ 1 │ 1 │ 1 │ 1 │ 7 │ 7 │  7 │  7 │  7 │  7 │  7
  ───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼────┼────┼────┼────┼────
  k3 │ 0 │ 1 │ 1 │ 1 │ 1 │ 5 │ 5 │ 7 │ 7 │  7 │ 10 │ 10 │ 10 │ 10

A linear time algorithm can be defined by using the formula for the
probability of ch(key, j) jumping when j increases. It essentially walks
across a row of this table. Given a key and number of buckets, the algorithm
considers each successive bucket, j, from 1 to num_buckets­1, and uses
ch(key, j) to compute ch(key, j+1). At each bucket, j, it decides whether to
keep ch(k, j+1) the same as ch(k, j), or to jump its value to j. In order to
jump for the right fraction of keys, it uses a pseudo­random number
generator with the key as its seed. To jump for 1/(j+1) of keys, it
generates a uniform random number between 0.0 and 1.0, and jumps if the
value is less than 1/(j+1). At the end of the loop, it has computed
ch(k, num_buckets), which is the desired answer. In code:

 int ch(int key, int num_buckets) {
   random.seed(key);
   int b = 0; // This will track ch(key,j+1).
   for (int j = 1; j < num_buckets; j++) {
     if (random.next() < 1.0 / (j + 1)) b = j;
   }
   return b;
 }

We can convert this to a logarithmic time algorithm by exploiting that
ch(key, j+1) is usually unchanged as j increases, only jumping occasionally.
The algorithm will only compute the destinations of jumps ­­ the j’s for
which ch(key, j+1) ≠ ch(key, j). Also notice that for these j’s, ch(key,
j+1) = j. To develop the algorithm, we will treat ch(key, j) as a random
variable, so that we can use the notation for random variables to analyze
the fractions of keys for which various propositions are true. That will
lead us to a closed form expression for a pseudo­random variable whose value
gives the destination of the next jump.

Suppose that the algorithm is tracking the bucket numbers of the jumps for a
particular key, k. And suppose that b was the destination of the last jump,
that is, ch(k, b) ≠ ch(k, b+1), and ch(k, b+1) = b. Now, we want to find the
next jump, the smallest j such that ch(k, j+1) ≠ ch(k, b+1), or
equivalently, the largest j such that ch(k, j) = ch(k, b+1). We will make a
pseudo­random variable whose value is that j. To get a probabilistic
constraint on j, note that for any bucket number, i, we have j ≥ i if and
only if the consistent hash hasn’t changed by i, that is, if and only if
ch(k, i) = ch(k, b+1). Hence, the distribution of j must satisfy

 P(j ≥ i) = P( ch(k, i) = ch(k, b+1) )

Fortunately, it is easy to compute that probability. Notice that since P(
ch(k, 10) = ch(k, 11) ) is 10/11, and P( ch(k, 11) = ch(k, 12) ) is 11/12,
then P( ch(k, 10) = ch(k, 12) ) is 10/11 * 11/12 = 10/12. In general, if n ≥
m, P( ch(k, n) = ch(k, m) ) = m / n. Thus for any i > b,

 P(j ≥ i) = P( ch(k, i) = ch(k, b+1) ) = (b+1) / i .

Now, we generate a pseudo­random variable, r, (depending on k and j) that is
uniformly distributed between 0 and 1. Since we want P(j ≥ i) = (b+1) / i,
we set P(j ≥ i) iff r ≤ (b+1) / i. Solving the inequality for i yields P(j ≥
i) iff i ≤ (b+1) / r. Since i is a lower bound on j, j will equal the
largest i for which P(j ≥ i), thus the largest i satisfying i ≤ (b+1) / r.
Thus, by the definition of the floor function, j = floor((b+1) / r).

Using this formula, jump consistent hash finds ch(key, num_buckets) by
choosing successive jump destinations until it finds a position at or past
num_buckets. It then knows that the previous jump destination is the answer.

 int ch(int key, int num_buckets) {
   random.seed(key);
   int b = -1; // bucket number before the previous jump
   int j = 0;  // bucket number before the current jump
     while (j < num_buckets) {
       b = j;
       r = random.next();
       j = floor((b + 1) / r);
   }
   return = b;
 }

To turn this into the actual code of figure 1, we need to implement random.
We want it to be fast, and yet to also to have well distributed successive
values. We use a 64­bit linear congruential generator; the particular
multiplier we use produces random numbers that are especially well
distributed in higher dimensions (i.e., when successive random values are
used to form tuples). We use the key as the seed. (For keys that don’t fit
into 64 bits, a 64 bit hash of the key should be used.) The congruential
generator updates the seed on each iteration, and the code derives a double
from the current seed. Tests show that this generator has good speed and
distribution.

It is worth noting that unlike the algorithm of Karger et al., jump
consistent hash does not require the key to be hashed if it is already an
integer. This is because jump consistent hash has an embedded pseudorandom
number generator that essentially rehashes the key on every iteration. The
hash is not especially good (i.e., linear congruential), but since it is
applied repeatedly, additional hashing of the input key is not necessary.

[1] http://arxiv.org/pdf/1406.2294v1.pdf
*/
package jump

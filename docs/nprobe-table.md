# Recall vs nprobe

Dataset: **siftsmall** (10,000 base vectors, 100 queries, 128 dims, L2), top-10.
Index: k-means, k = 100 clusters (sqrt(n)), 25 rounds max. k-means took ~3.7 s.
Run it with `cd scratchpad/nprobe && go run .` (pass `../../testdata/sift/sift` for SIFT1M).

Brute force: recall@10 = **1.0000**, ~1.40 ms per query. (With cosine instead of L2 it was 0.9940, because SIFT's ground truth uses L2.)

| nprobe | recall@10 | time per query |
| --- | --- | --- |
| 1 | 0.5800 | 35 µs |
| 5 | 0.9050 | 83 µs |
| 10 | 0.9730 | 142 µs |
| 20 | 0.9960 | 252 µs |
| 50 | 1.0000 | 531 µs |

Why recall goes up with `nprobe`: a vector's true neighbour often sits in a *neighbouring*
cluster, because a query near a cluster border has near neighbours on both sides of the border.
Probing more clusters catches those. The cost is time: each extra cluster means ~n/k more
distance computations, so time grows roughly linearly with `nprobe`. At nprobe = 50 we check half
the data and are only about 2.6x faster than brute force.

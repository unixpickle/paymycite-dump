# paymycite-dump

I got a parking ticket in Sonoma County, so naturally I am now dumping a large random sample of real parking citations from their online database.

# How it works

The website [paymycite.com](paymycite.com) allows you to search a parking ticket using a combination of the issuing agency (e.g. "Sonoma County") and a six-digit citation number. After initial investigation, it looks like there are 166 valid agencies. Unfortunately, it seems that both fields need to be exactly correct for a search result to appear. This means that the total search space has roughly `166*10^6 = 166,000,000` permutations.

After some initial searching, one can compute some statistics about the search space. A random query has roughly `0.00786` probability of a hit, so we must check about 130 random records to uncover a valid one. From this we can estimate that there are about 1.3M records to be found in total.

For some citations, the website also provides a way to contest the citation. The page for this displays a lot of extra metadata, like the address where the citation occurred and the nature of the violation. Note that this metadata can be fetched without actually submitting the form.

# Running

First, you can randomly scan for citation data using the `find_cites` script, which will continually dump citations as they are found:

```
go run ./find_cites
```

This will continually write to a CSV file `output.csv` with these columns:

 * agency
 * citation number
 * agency name
 * license plate
 * state
 * date
 * total fine
 * notes (usually just empty or a phone number)

Some citations have additional metadata available through a "contest" form, such as an address and violation code. To fill in this info, run the `fetch_details` script like so (after running `find_cites`):

```
go run ./fetch_details
```

This script goes through records in `output.csv` and writes augmented records to a file `details.csv`. The resulting CSV has a header row listing all of the field names.

Reference vectors for the HF model (internal/hf).

caseNN.in / xNN.in   input files of the ITU-R reference program ITURHFProp (C). The machine-specific paths are tokens:
                     @DATA@ (data directory) and @TESTDATA@ (this directory); the Go test ignores DataFilePath and
                     takes antenna files from this directory by name
caseNN.csv / xNN.csv the CSV report that program wrote for them (71 columns; the first lines carry a build timestamp)
*.t13 *.t11 *.t14    antenna files used by case04, case10, x04 and x05 (141-14_0.t13 comes from the ITU-R-HF repository,
                     syn11.t11 and syn14.t14 are synthetic)

TestReferenceCases runs every input through Predict and requires every report column to be identical when printed with
the decimals of the reference. The data set itself is fetched by scripts/fetch-hf-data.sh.

Regenerate: build ITURHFProp from the P533/P372/ITURHFProp sources of
https://github.com/ITU-R-Study-Group-3/ITU-R-HF (unmodified, gcc; on Windows -fcommon replaces -z muldefs and
MakeNoise.c is added to the P372 library), then run
  internal/hf/testdata/regen.sh PATH/TO/ITURHFProp tmp/testfiles/hf1
which fills in the tokens on a temporary copy of every .in file and rewrites the .csv files.
p372/testdata/noise_vectors.csv was made the same way from MakeNoise.c.

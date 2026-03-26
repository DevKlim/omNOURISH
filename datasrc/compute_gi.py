import pandas as pd

TOTPOP_R = 0.25
MEDVAL_R = 0.25
MEDHINC_R = 0.25
MEDCRNT_R = 0.25

def rank_enrich_val(df, col):
    """
    rank the enrichment value within current scope (San Diego)
    """
    return df[col].rank(pct=True, method="average")

def compute_rank_delta(df, t0_col, t1_col):
    t0_rank = rank_enrich_val(df, t0_col)
    t1_rank = rank_enrich_val(df, t1_col)
    return t1_rank - t0_rank


def compute_gi_score(df):
    # rename the columns accordingly to available dataset
    # updated to census renamed data column names
    totpop_delta = compute_rank_delta(df, 'totpop23', 'totpop19')
    medval_delta = compute_rank_delta(df, 'medhval23', 'medhval19')
    medhinc_delta = compute_rank_delta(df, 'medhinc23', 'medhinc19')
    medcrnt_delta = compute_rank_delta(df, 'medrent23', 'medrent19')
    return TOTPOP_R*totpop_delta + MEDVAL_R*medval_delta + MEDHINC_R*medhinc_delta + MEDCRNT_R*medcrnt_delta
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
    totpop_delta = compute_rank_delta(df, 'TOTPOP_C_1', 'TOTPOP_F_1')
    medval_delta = compute_rank_delta(df, 'MEDVAL_C_1', 'MEDVAL_F_1')
    medhinc_delta = compute_rank_delta(df, 'MEDHINC__1', 'MEDHINC__2')
    medcrnt_delta = compute_rank_delta(df, 'MEDCRNT_CY', 'MEDCRNT_FY')
    return TOTPOP_R*totpop_delta + MEDVAL_R*medval_delta + MEDHINC_R*medhinc_delta + MEDCRNT_R*medcrnt_delta
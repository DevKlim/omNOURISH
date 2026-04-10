from pygris.data import get_census
import pygris
import numpy as np
import pandas as pd
from compute_gi import compute_gi_score


def get_crosswalk_val(crosswalk_col, val_col, old_df, new_df, agg=False):
    out = []
    for tr_ge in new_df.GEOID:
        crosswalk_dict = ct_mapper[ct_mapper['tr2020ge'] == tr_ge][['tr2012ge', crosswalk_col]].set_index('tr2012ge')
        out_val = sum((old_df.set_index('GEOID').loc[crosswalk_dict.index, val_col]*crosswalk_dict[crosswalk_col].astype(float)).dropna())
        if not agg:
            try:
                out_val = out_val/sum(crosswalk_dict[crosswalk_col].astype(float))
            except:
                out_val = 0
        out.append(np.round(out_val))

    return out

if __name__ == '__main__':

    # DP03_0062E: Estimate!!INCOME AND BENEFITS (IN 2019 INFLATION-ADJUSTED DOLLARS)!!Total households!!Median household income (dollars)
    # DP04_0089E: Estimate!!VALUE!!Owner-occupied units!!Median (dollars)
    # DP04_0134E: Estimate!!GROSS RENT!!Occupied units paying rent!!Median (dollars)
    # DP05_0001E: Estimate!!SEX AND AGE!!Total population
    variables = ['DP03_0062E', 'DP04_0089E', 'DP04_0134E', 'DP05_0001E']
    
    ca_gentrification_vars_23 = get_census(dataset = "acs/acs5/profile",
                            variables = variables,
                            year = 2023,
                            params = {
                              "for": "tract:*",
                              "in": "state:06 county:073"},
                            guess_dtypes = True,
                            return_geoid = True)
    
    ca_gentrification_vars_23 = ca_gentrification_vars_23.rename({'DP03_0062E': 'medhinc23', 
                                                                  'DP04_0089E': 'medhval23', 
                                                                  'DP04_0134E': 'medrent23', 
                                                                  'DP05_0001E': 'totpop23'}, axis=1)
    
    
    ca_gentrification_vars_19 = get_census(dataset = "acs/acs5/profile",
                            variables = variables,
                            year = 2019,
                            params = {
                              "for": "tract:*",
                              "in": "state:06 county:073"},
                            guess_dtypes = True,
                            return_geoid = True)
    
    ca_gentrification_vars_19 = ca_gentrification_vars_19.rename({'DP03_0062E': 'medhinc19', 
                                                                  'DP04_0089E': 'medhval19', 
                                                                  'DP04_0134E': 'medrent19', 
                                                                  'DP05_0001E': 'totpop19'}, axis=1)
    
    
    ct_mapper = pd.read_csv(r'..\data\ctmapping\nhgis_tr2012_tr2020_06.csv', dtype=str)

    ca_gentrification_vars_23['totpop19'] = get_crosswalk_val('wt_pop', 'totpop19', ca_gentrification_vars_19, ca_gentrification_vars_23, agg=True)
    ca_gentrification_vars_23['medhval19'] = get_crosswalk_val('wt_ownhu', 'medhval19', ca_gentrification_vars_19, ca_gentrification_vars_23)
    ca_gentrification_vars_23['medrent19'] = get_crosswalk_val('wt_renthu', 'medrent19', ca_gentrification_vars_19, ca_gentrification_vars_23)
    ca_gentrification_vars_23['medhinc19'] = get_crosswalk_val('wt_hh', 'medhinc19', ca_gentrification_vars_19, ca_gentrification_vars_23)
    ca_gentrification_vars_23['GENTRIFICATION'] = compute_gi_score(ca_gentrification_vars_23)

    ca_gentrification_vars_23.to_csv('..\data\giscore_enriched\census_gentrification.csv')
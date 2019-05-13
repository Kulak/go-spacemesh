import os
import re
import time
from datetime import datetime

from elasticsearch import Elasticsearch
from elasticsearch_dsl import Search, Q

dt = datetime.now()
todaydate = dt.strftime("%Y.%m.%d")
current_index = 'kubernetes_cluster-' + todaydate


def get_elastic_search_api():
    # TODO: convert to a singleton, preferably from main test runner. (maybe just get the api as param for queries?)
    ES_PASSWD = os.getenv("ES_PASSWD")
    if not ES_PASSWD:
        raise Exception("Unknown Elasticsearch password. Please check 'ES_PASSWD' environment variable")
    es = Elasticsearch("http://elastic.spacemesh.io",
                       http_auth=("spacemesh", ES_PASSWD), port=80)
    return es


def get_podlist(namespace, depname):
    api = get_elastic_search_api()
    fltr = Q("match_phrase", kubernetes__pod_name=depname) & Q("match_phrase", kubernetes__namespace_name=namespace)
    s = Search(index=current_index, using=api).query('bool').filter(fltr)
    hits = list(s.scan())
    podnames = set([hit.kubernetes.pod_name for hit in hits])
    return podnames


def get_pod_logs(namespace, pod_name):
    api = get_elastic_search_api()
    fltr = Q("match_phrase", kubernetes__pod_name=pod_name) & Q("match_phrase", kubernetes__namespace_name=namespace)
    s = Search(index=current_index, using=api).query('bool').filter(fltr).sort("time")
    res = s.execute()
    full = Search(index=current_index, using=api).query('bool').filter(fltr).sort("time").extra(size=res.hits.total)
    res = full.execute()
    hits = list(res.hits)
    print("Writing ${0} log lines for pod {1} ".format(len(hits), pod_name))
    with open('./logs/' + pod_name + '.txt', 'w') as f:
        for i in hits:
            f.write(i.log)


def get_podlist_logs(namespace, podlist):
    for i in podlist:
        get_pod_logs(namespace, i)


def get_deployment_logs(namespace, depname):
    lst = get_podlist(namespace, depname)
    print("Getting pod list ", lst)
    get_podlist_logs(namespace, lst)


def query_message(indx, namespace, client_po_name, fields, findFails=False):
    # TODO : break this to smaller functions ?
    es = get_elastic_search_api()
    fltr = Q("match_phrase", kubernetes__namespace_name=namespace) & \
           Q("match_phrase", kubernetes__pod_name=client_po_name)
    for f in fields:
        fltr = fltr & Q("match_phrase", **{f: fields[f]})
    s = Search(index=indx, using=es).query('bool', filter=[fltr])
    hits = list(s.scan())

    print("====================================================================")
    print("Report for `{0}` in deployment -  {1}  ".format(fields, client_po_name))
    print("Number of hits: ", len(hits))
    print("====================================================================")
    print("Benchmark results:")
    if len(hits) > 0:
        ts = [hit["T"] for hit in hits]

        first = datetime.strptime(min(ts).replace("T", " ", ).replace("Z", ""), "%Y-%m-%d %H:%M:%S.%f")
        last = datetime.strptime(max(ts).replace("T", " ", ).replace("Z", ""), "%Y-%m-%d %H:%M:%S.%f")

        delta = last - first
        print("First: {0}, Last: {1}, Delta: {2}".format(first, last, delta))
        # TODO: compare to previous runs.
        print("====================================================================")
    else:
        print("no hits")
    if findFails:
        print("Looking for pods that didn't hit:")
        podnames = [hit.kubernetes.pod_name for hit in hits]
        newfltr = Q("match_phrase", kubernetes__namespace_name=namespace) & \
                  Q("match_phrase", kubernetes__pod_name=client_po_name)

        for p in podnames:
            newfltr = newfltr & ~Q("match_phrase", kubernetes__pod_name=p)
        s2 = Search(index=current_index, using=es).query('bool', filter=[newfltr])
        hits2 = list(s2.scan())
        unsecpods = set([hit.kubernetes.pod_name for hit in hits2])
        if len(unsecpods) == 0:
            print("None. yay!")
        else:
            print(unsecpods)
        print("====================================================================")

    s = list([(h.N, h.M) for h in hits])
    return s


def parseAtx(log_messages):
    node2blocks = {}
    for x in log_messages:
        nid = re.split(r'\.', x[0])[0]
        m = re.findall(r'(?:[0-9][^, ]*[A-Za-z][^, ]*)|(?:[A-Za-z][^, ]*[0-9][^, ]*)|(?:\d+)', x[1])
        if nid in node2blocks:
            node2blocks[nid].append(m)
        else:
            node2blocks[nid] = [m]
    return node2blocks

def sort_by_nodeid(log_messages):
    node2blocks = {}
    for x in log_messages:
        id = re.split(r'\.', x[0])[0]
        m = re.findall(r'\d+', x[1])
        # re.search(r'(?<=in layer\s)\d+', x[1])
        if id in node2blocks:
            node2blocks[id].append(m)
        else:
            node2blocks[id] = [m]
    return node2blocks


def get_blocks_per_node(deployment):
    # I've created a block in layer %v. id: %v, num of transactions: %v, votes: %d, viewEdges: %d atx %v, atxs:%v
    block_fields = {"M": "I've created a block in layer"}
    blocks = query_message(current_index, deployment, deployment, block_fields, True)
    print("found " + str(len(blocks)) + " blocks")
    nodes = sort_by_nodeid(blocks)

    return nodes


def get_layers(deployment):
    block_fields = {"M": "release tick"}
    layers = query_message(current_index, deployment, deployment, block_fields, True)
    ids = [int(re.findall(r'\d+', x[1])[0]) for x in layers]
    return ids


def get_latest_layer(deployment):
    layers = get_layers(deployment)
    layers.sort(reverse=True)
    return layers[0]


def wait_for_latest_layer(deployment, layer_id):
    while True:
        lyr = get_latest_layer(deployment)
        if lyr < layer_id:
            print("current layer " + str(lyr))
            time.sleep(10)
        else:
            return


def get_atx_per_node(deployment):
    # based on log: atx published! id: %v, prevATXID: %v, posATXID: %v, layer: %v, published in epoch: %v, active set: %v miner: %v view %v
    block_fields = {"M": "atx published"}
    atx_logs = query_message(current_index, deployment, deployment, block_fields, True)
    print("found " + str(len(atx_logs)) + " atxs")
    nodes = parseAtx(atx_logs)

    return nodes

